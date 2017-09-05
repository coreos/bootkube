package util

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

var DefaultBackoff = wait.Backoff{
	Steps:    10,
	Duration: 5 * time.Second,
	Factor:   1.5,
	Jitter:   0.1,
}

func Retry(backoff wait.Backoff, fn func() error) error {
	var lastConflictErr error
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := fn()
		switch {
		case err == nil:
			return true, nil
		default:
			lastConflictErr = err
			return false, nil
		}
	})
	if err == wait.ErrWaitTimeout {
		err = lastConflictErr
	}
	return err
}
