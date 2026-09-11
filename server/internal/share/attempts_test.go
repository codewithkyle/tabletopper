package share_test

import (
	"testing"
	"time"

	"tabletopper/internal/share"
)

func TestAShareStopsAnsweringOnceItsTriesAreSpent(t *testing.T) {
	attempts := share.NewAttempts(10, time.Minute)
	now := time.Now()

	for i := range 10 {
		if !attempts.Allow("tokenA", now) {
			t.Fatalf("try %d was refused inside the limit", i+1)
		}
	}
	if attempts.Allow("tokenA", now) {
		t.Error("the eleventh try was allowed")
	}
}



func TestOneLockedShareDoesNotLockAnother(t *testing.T) {
	attempts := share.NewAttempts(2, time.Minute)
	now := time.Now()

	attempts.Allow("tokenA", now)
	attempts.Allow("tokenA", now)
	if attempts.Allow("tokenA", now) {
		t.Fatal("tokenA was not locked after its limit")
	}

	if !attempts.Allow("tokenB", now) {
		t.Error("tokenB was refused because tokenA was attacked")
	}
}

func TestTheWindowReopens(t *testing.T) {
	attempts := share.NewAttempts(2, time.Minute)
	start := time.Now()

	attempts.Allow("tokenA", start)
	attempts.Allow("tokenA", start)
	if attempts.Allow("tokenA", start) {
		t.Fatal("the third try in the window was allowed")
	}

	if !attempts.Allow("tokenA", start.Add(time.Minute)) {
		t.Error("a try after the window was still refused")
	}
}



func TestARefusedTryStillCounts(t *testing.T) {
	attempts := share.NewAttempts(1, time.Minute)
	start := time.Now()

	attempts.Allow("tokenA", start)
	for i := range 5 {
		if attempts.Allow("tokenA", start.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("a try %ds into a locked window was allowed", i)
		}
	}
}
