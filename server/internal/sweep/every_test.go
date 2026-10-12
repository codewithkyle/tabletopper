package sweep

import (
	"context"
	"testing"
	"time"
)

func TestALoopRunsAtOnceOnEveryWakeAndStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runs := make(chan struct{}, 8)
	loop := Every(ctx, "Test loop", time.Hour, func(context.Context) { runs <- struct{}{} })
	waitFor := func(what string) {
		t.Helper()
		select {
		case <-runs:
		case <-time.After(2 * time.Second):
			t.Fatalf("the loop did not run %s", what)
		}
	}
	waitFor("at once")
	loop.Wake()
	waitFor("when woken")
	cancel()
	select {
	case <-loop.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the loop did not stop with its context")
	}
	if len(runs) != 0 {
		t.Errorf("the loop ran %d more times than it was asked to", len(runs))
	}
}
func TestALoopRunsOnEveryTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runs := make(chan struct{}, 8)
	Every(ctx, "Test loop", 10*time.Millisecond, func(context.Context) { runs <- struct{}{} })
	for i := range 3 {
		select {
		case <-runs:
		case <-time.After(2 * time.Second):
			t.Fatalf("run %d never came", i+1)
		}
	}
}
