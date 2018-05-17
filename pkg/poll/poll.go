// Package poll supports polling until a condition is satisfied or a
// context finishes.
package poll

import (
	"context"
	"time"
)

// ConditionFunc returns true if the condition is satisfied, or an
// error if the loop should be aborted.
type ConditionFunc func(ctx context.Context) (done bool, err error)

// Poll tries a condition func until it returns true, an error, or the
// context is done.
//
// If the context is done before interval elapses, Poll will not call
// 'condition' at all.
//
// Some intervals may be missed if the condition takes too long or the time
// window is too short.
func Poll(ctx context.Context, interval time.Duration, condition ConditionFunc) (err error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ok, err := condition(ctx)
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// PollImmediate is like Poll, except it tries the condition
// immediately without waiting for the first interval.
func PollImmediate(ctx context.Context, interval time.Duration, condition ConditionFunc) (err error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ok, err := condition(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	for {
		select {
		case <-ticker.C:
			ok, err := condition(ctx)
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
