package util

import (
	"flag"
	"log"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
)

type GlogWriter struct{}

func init() {
	flag.Set("logtostderr", "true")
}

func (writer GlogWriter) Write(data []byte) (n int, err error) {
	glog.Info(string(data))
	return len(data), nil
}

func InitLogs() {
	log.SetOutput(GlogWriter{})
	log.SetFlags(0)
	flushFreq := 5 * time.Second
	go wait.Until(glog.Flush, flushFreq, wait.NeverStop)
}

func FlushLogs() {
	glog.Flush()
}
