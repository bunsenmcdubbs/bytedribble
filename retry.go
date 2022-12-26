package bytedribble

import (
	"context"
	"errors"
	"time"
)

func RetryWithExpBackoff(ctx context.Context, f func(context.Context) error, delay time.Duration, maxTries int) error {
	t := time.NewTimer(0)
	defer t.Stop()
	for attemptNum := 0; attemptNum < maxTries; attemptNum++ {
		if err := f(ctx); err == nil || errors.Is(err, context.Canceled) {
			return err
		}
		t.Reset(delay)
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		delay *= 2
	}
	return errors.New("exceeded max retries")
}
