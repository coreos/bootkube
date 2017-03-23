package upload

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Microsoft/azure-vhd-utils-for-go/upload/concurrent"
	"github.com/Microsoft/azure-vhd-utils-for-go/upload/progress"
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/common"
	"github.com/Microsoft/azure-vhd-utils-for-go/vhdcore/diskstream"
	"io"
	"time"
)

// DiskUploadContext type describes VHD upload context, this includes the disk stream to read from, the ranges of
// the stream to read, the destination blob and it's container, the client to communicate with Azure storage and
// the number of parallel go-routines to use for upload.
//
type DiskUploadContext struct {
	VhdStream             *diskstream.DiskStream    // The stream whose ranges needs to be uploaded
	AlreadyProcessedBytes int64                     // The size in bytes already uploaded
	UploadableRanges      []*common.IndexRange      // The subset of stream ranges to be uploaded
	BlobServiceClient     storage.BlobStorageClient // The client to make Azure blob service API calls
	ContainerName         string                    // The container in which page blob resides
	BlobName              string                    // The destination page blob name
	Parallelism           int                       // The number of concurrent goroutines to be used for upload
	Resume                bool                      // Indicate whether this is a new or resuming upload
	MD5Hash               []byte                    // MD5Hash to be set in the page blob properties once upload finishes
}

// oneMB is one MegaByte
//
const oneMB = float64(1048576)

// Upload uploads the disk ranges described by the parameter cxt, this parameter describes the disk stream to
// read from, the ranges of the stream to read, the destination blob and it's container, the client to communicate
// with Azure storage and the number of parallel go-routines to use for upload.
//
func Upload(cxt *DiskUploadContext) error {
	// Get the channel that contains stream of disk data to upload
	dataWithRangeChan, streamReadErrChan := GetDataWithRanges(cxt.VhdStream, cxt.UploadableRanges)

	// The channel to send upload request to load-balancer
	requtestChan := make(chan *concurrent.Request, 0)

	// Prepare and start the load-balancer that load request across 'cxt.Parallelism' workers
	loadBalancer := concurrent.NewBalancer(cxt.Parallelism)
	loadBalancer.Init()
	workerErrorChan, allWorkersFinishedChan := loadBalancer.Run(requtestChan)

	// Calculate the actual size of the data to upload
	uploadSizeInBytes := int64(0)
	for _, r := range cxt.UploadableRanges {
		uploadSizeInBytes += r.Length()
	}

	fmt.Printf("\nEffective upload size: %.2f MB (from %.2f MB originally)", float64(uploadSizeInBytes)/oneMB, float64(cxt.VhdStream.GetSize())/oneMB)

	// Prepare and start the upload progress tracker
	uploadProgress := progress.NewStatus(cxt.Parallelism, cxt.AlreadyProcessedBytes, uploadSizeInBytes, progress.NewComputestateDefaultSize())
	progressChan := uploadProgress.Run()
	// read progress status from progress tracker and print it
	go readAndPrintProgress(progressChan, cxt.Resume)

	go func() {
		for {
			fmt.Println("FailedAfterAllRetries:", <-workerErrorChan)
		}
	}()

	var err error
L:
	for {
		select {
		case dataWithRange, ok := <-dataWithRangeChan:
			if !ok {
				close(requtestChan)
				break L
			}

			req := &concurrent.Request{
				Work: func() error {
					err := cxt.BlobServiceClient.PutPage(cxt.ContainerName,
						cxt.BlobName,
						dataWithRange.Range.Start,
						dataWithRange.Range.End,
						storage.PageWriteTypeUpdate,
						dataWithRange.Data,
						nil)
					if err == nil {
						uploadProgress.ReportBytesProcessedCount(dataWithRange.Range.Length())
					}
					return err
				},
				ShouldRetry: func(e error) bool {
					return true
				},
				ID: dataWithRange.Range.String(),
			}

			requtestChan <- req // Send to load balancer
		case err = <-streamReadErrChan:
			close(requtestChan)
			loadBalancer.TearDownWorkers()
			break L
		}
	}

	<-allWorkersFinishedChan
	uploadProgress.Close()
	if err == nil {
		// TODO: Set MD5 Hash for the PageBlob, the Storage SDK does not implement method to set BlobProperties
		// https://msdn.microsoft.com/en-us/library/azure/ee691966.aspx
		fmt.Printf("\r Completed: %3d%% [%10.2f MB] RemainingTime: %02dh:%02dm:%02ds Throughput: %d MB/sec  %2c ",
			100,
			float64(uploadSizeInBytes)/oneMB,
			0, 0, 0,
			0, ' ')

	}
	return err
}

// GetDataWithRanges with start reading and streaming the ranges from the disk identified by the parameter ranges.
// It returns two channels, a data channel to stream the disk ranges and a channel to send any error while reading
// the disk. On successful completion the data channel will be closed. the caller must not expect any more value in
// the data channel if the error channel is signaled.
//
func GetDataWithRanges(stream *diskstream.DiskStream, ranges []*common.IndexRange) (<-chan *DataWithRange, <-chan error) {
	dataWithRangeChan := make(chan *DataWithRange, 0)
	errorChan := make(chan error, 0)
	go func() {
		for _, r := range ranges {
			dataWithRange := &DataWithRange{
				Range: r,
				Data:  make([]byte, r.Length()),
			}
			_, err := stream.Seek(r.Start, 0)
			if err != nil {
				errorChan <- err
				return
			}
			_, err = io.ReadFull(stream, dataWithRange.Data)
			if err != nil {
				errorChan <- err
				return
			}
			dataWithRangeChan <- dataWithRange
		}
		close(dataWithRangeChan)
	}()
	return dataWithRangeChan, errorChan
}

// readAndPrintProgress reads the progress records from the given progress channel and output it. It reads the
// progress record until the channel is closed.
//
func readAndPrintProgress(progressChan <-chan *progress.Record, resume bool) {
	var spinChars = [4]rune{'\\', '|', '/', '-'}
	s := time.Time{}
	if resume {
		fmt.Println("\nResuming VHD upload..")
	} else {
		fmt.Println("\nUploading the VHD..")
	}

	i := 0
	for progressRecord := range progressChan {
		if i == 4 {
			i = 0
		}
		t := s.Add(progressRecord.RemainingDuration)
		fmt.Printf("\r Completed: %3d%% [%10.2f MB] RemainingTime: %02dh:%02dm:%02ds Throughput: %d MB/sec  %2c ",
			int(progressRecord.PercentComplete),
			float64(progressRecord.BytesProcessed)/oneMB,
			t.Hour(), t.Minute(), t.Second(),
			int(progressRecord.AverageThroughputMBPerSecond),
			spinChars[i],
		)
		i++
	}
}
