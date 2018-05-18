package util

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/bootkube/pkg/poll"
)

type GlogWriter struct{}

func init() {
	flag.Set("logtostderr", "true")
}

func (writer GlogWriter) Write(data []byte) (n int, err error) {
	glog.Info(string(data))
	return len(data), nil
}

func InitLogs(ctx context.Context, sync chan<- error) {
	log.SetOutput(GlogWriter{})
	log.SetFlags(log.LUTC | log.Ldate | log.Ltime)
	go func() {
		err := poll.Poll(ctx, 5*time.Second, func(ctx context.Context) (ok bool, err error) {
			glog.Flush()
			return false, nil
		})
		if err != nil {
			glog.Flush()
			sync <- err
			return
		}
	}()
}
