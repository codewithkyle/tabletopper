package sweep

import (
	"context"
	"log/slog"
	"time"
)

type Loop struct {
	wake chan struct{}
	done chan struct{}
}

func Every(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) *Loop {
	l := &Loop{wake: make(chan struct{}, 1), done: make(chan struct{})}
	go func() {
		defer close(l.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		slog.Info(name+" started", "every", interval.String())
		fn(ctx)
		for {
			select {
			case <-ctx.Done():
				slog.Info(name + " stopped")
				return
			case <-l.wake:
				fn(ctx)
			case <-ticker.C:
				fn(ctx)
			}
		}
	}()
	return l
}
func (l *Loop) Wake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}
func (l *Loop) Done() <-chan struct{} { return l.done }
