package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// The mapping the music confirm's rollback decision rests on. A HEAD's 404 and
// a GET's 404 are two different SDK types, and a caller that only matched one
// would discard a landed 175 MB upload the first time the other came back.
func TestBothShapesOfAMissingObjectMapToErrNotFound(t *testing.T) {
	for name, err := range map[string]error{
		"head": &types.NotFound{},
		"get":  &types.NoSuchKey{},
	} {
		t.Run(name, func(t *testing.T) {
			if got := notFound(err); !errors.Is(got, ErrNotFound) {
				t.Errorf("notFound(%T) = %v, want it to wrap ErrNotFound", err, got)
			}
		})
	}
}

// And nothing else does. A 5xx, a timeout or a dropped connection means the
// object is probably there and only the question failed, which is the case the
// sentinel exists to keep apart.
func TestEveryOtherFailureIsLeftAlone(t *testing.T) {
	for name, err := range map[string]error{
		"server error": errors.New("api error InternalError: We encountered an internal error"),
		"timeout":      context.DeadlineExceeded,
		"cancelled":    context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			got := notFound(err)
			if errors.Is(got, ErrNotFound) {
				t.Errorf("notFound(%v) wrapped ErrNotFound", err)
			}
			if !errors.Is(got, err) {
				t.Errorf("notFound(%v) = %v, want the error unchanged", err, got)
			}
		})
	}
}
