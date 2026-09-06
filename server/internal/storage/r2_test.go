package storage

import (
	"context"
	"testing"
)

// THE CLIENT HERE HAS NO s3 FIELD, and that is the assertion. Every one of
// these returns before anything is sent, so a client that could not reach R2 --
// or in this case could not reach anything at all -- is enough to run them. If a
// guard were dropped the call would panic on a nil pointer rather than fail.

// The sweeper calls this with whatever a batch came back with, and a quiet hour
// is an empty batch. Sending a DeleteObjects with no objects in it would be a
// round trip per hour to say nothing.
func TestDeleteManyWithNoKeysDoesNothing(t *testing.T) {
	c := &Client{bucket: "test"}

	if err := c.DeleteMany(context.Background(), nil); err != nil {
		t.Errorf("DeleteMany(nil) = %v, want nil", err)
	}
	if err := c.DeleteMany(context.Background(), []string{}); err != nil {
		t.Errorf("DeleteMany([]) = %v, want nil", err)
	}
}

// An empty key is refused for the whole batch and before any of it is sent, the
// same way Delete refuses one. R2 would take it as the bucket root.
func TestDeleteManyRefusesAnEmptyKey(t *testing.T) {
	c := &Client{bucket: "test"}

	if err := c.DeleteMany(context.Background(), []string{"users/a/journals/b", ""}); err == nil {
		t.Error("DeleteMany with an empty key = nil, want an error")
	}
}

// The guard on DeletePrefix is the one in this package whose absence would be
// destructive rather than merely wrong: without it the empty string, which is
// what a prefix built from a zero-valued id comes to, lists and deletes the
// whole bucket. A prefix that is missing its trailing slash is refused for a
// smaller version of the same reason -- it is a string match, and it reaches
// past the directory it was meant to name.
func TestDeletePrefixRefusesAnythingThatIsNotADirectory(t *testing.T) {
	c := &Client{bucket: "test"}

	for _, prefix := range []string{"", "users/a/maps/01JB", "users/a/maps/01JB/original"} {
		if err := c.DeletePrefix(context.Background(), prefix); err == nil {
			t.Errorf("DeletePrefix(%q) = nil, want an error", prefix)
		}
	}
}
