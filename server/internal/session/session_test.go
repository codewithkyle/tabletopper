package session

import (
	"testing"
	"time"
)

func TestNextExpirySlidesForwardWhileYoung(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	now := created.Add(24 * time.Hour)

	if got, want := nextExpiry(now, created), now.Add(IdleWindow); !got.Equal(want) {
		t.Errorf("nextExpiry = %v, want %v", got, want)
	}
}

func TestNextExpiryClampsToMaxLifetime(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cap := created.Add(MaxLifetime)

	// a refresh landing just inside the cap must not extend a full idle window past it
	now := cap.Add(-time.Minute)

	got := nextExpiry(now, created)
	if !got.Equal(cap) {
		t.Errorf("nextExpiry = %v, want the cap %v", got, cap)
	}
	if got.After(cap) {
		t.Errorf("nextExpiry = %v exceeds the cap %v", got, cap)
	}
}

func TestNextExpiryNeverExceedsCapAcrossLifetime(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cap := created.Add(MaxLifetime)

	for d := time.Duration(0); d < MaxLifetime; d += 12 * time.Hour {
		if got := nextExpiry(created.Add(d), created); got.After(cap) {
			t.Fatalf("at +%s nextExpiry = %v, exceeds cap %v", d, got, cap)
		}
	}
}
