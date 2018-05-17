package poll

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestPollSecondMatch(t *testing.T) {
	ctx := context.Background()
	i := 0
	condition := func(ctx context.Context) (ok bool, err error) {
		i++
		if i == 2 {
			return true, nil
		}
		return false, nil
	}
	interval := time.Second
	start := time.Now()
	err := Poll(ctx, interval, condition)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Now().Sub(start)
	if math.Abs(elapsed.Seconds()/interval.Seconds()-2) > 0.2 {
		t.Fatalf("expected 2 seconds, got %f", elapsed.Seconds())
	}
}

func TestPollCancelMatch(t *testing.T) {
	timeout := time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	condition := func(ctx context.Context) (ok bool, err error) {
		return false, nil
	}
	start := time.Now()
	err := Poll(ctx, 2*timeout, condition)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected %v, got %v", context.DeadlineExceeded, err)
	}
	elapsed := time.Now().Sub(start)
	if math.Abs(elapsed.Seconds()/timeout.Seconds()-1) > 0.2 {
		t.Fatalf("expected %f seconds, got %f", timeout.Seconds(), elapsed.Seconds())
	}
}

func TestPollConditionError(t *testing.T) {
	ctx := context.Background()
	conditionError := errors.New("condition error")
	condition := func(ctx context.Context) (ok bool, err error) {
		return false, conditionError
	}
	interval := time.Second
	start := time.Now()
	err := Poll(ctx, interval, condition)
	if err != conditionError {
		t.Fatalf("expected %v, got %v", conditionError, err)
	}
	elapsed := time.Now().Sub(start)
	if math.Abs(elapsed.Seconds()/interval.Seconds()-1) > 0.2 {
		t.Fatalf("expected %f seconds, got %f", interval.Seconds(), elapsed.Seconds())
	}
}

func TestPollImmediateSecondMatch(t *testing.T) {
	ctx := context.Background()
	i := 0
	condition := func(ctx context.Context) (ok bool, err error) {
		time.Sleep(500 * time.Millisecond)
		i++
		if i == 2 {
			return true, nil
		}
		return false, nil
	}
	interval := time.Second
	start := time.Now()
	err := PollImmediate(ctx, interval, condition)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Now().Sub(start)
	if math.Abs(elapsed.Seconds()/interval.Seconds()-1.5) > 0.2 {
		t.Fatalf("expected 1.5 seconds, got %f", elapsed.Seconds())
	}
}
