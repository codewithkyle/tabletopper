package storage

import (
	"context"
	"testing"
)

// THE CLIENT HERE HAS NO s3 FIELD, and that is the assertion. Both of these
// return before anything is sent, so a client that could not reach R2 -- or in
// this case could not reach anything at all -- is enough to run them. If either
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
