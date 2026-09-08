package storage

import (
	"context"
	"errors"
	"net/http"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
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

// THE ONE THING THIS FIX IS is a classification, so it is the classification
// that is pinned. A map is tiled through sixteen concurrent PUTs and R2 answers
// a burst of them with 429 ServiceUnavailable, which the SDK's own retryer does
// not consider retryable -- neither the status nor the code is in its default
// sets -- so before this the first throttled tile abandoned the whole pyramid.
// Nothing about that is visible from the outside: a retryer that quietly stopped
// retrying would look exactly like R2 having a bad afternoon.
func TestR2ThrottlingIsRetried(t *testing.T) {
	for name, err := range map[string]error{
		"the code R2 sends":   &smithy.GenericAPIError{Code: "ServiceUnavailable", Message: "Reduce your concurrent request rate for the same object."},
		"the status it sends": throttleResponse(http.StatusTooManyRequests),
	} {
		t.Run(name, func(t *testing.T) {
			if !newRetryer().IsErrorRetryable(err) {
				t.Errorf("a throttled request is not retried: %v", err)
			}
		})
	}
}

// And the checks that were already there still are: adding to a set is only safe
// if it is an addition. A 503 is the case the SDK always handled and a 404 is the
// case it must go on refusing, because retrying a key that is not there is four
// more round trips to be told the same thing.
func TestTheSDKsOwnRetryRulesSurvive(t *testing.T) {
	if !newRetryer().IsErrorRetryable(throttleResponse(http.StatusServiceUnavailable)) {
		t.Error("a 503 is no longer retried")
	}
	if newRetryer().IsErrorRetryable(throttleResponse(http.StatusNotFound)) {
		t.Error("a 404 is retried")
	}
}

// A RETRY IS BOUNDED, and the bound is the reason it is safe to have on the
// client every request goes through rather than around the tiler's PUT alone.
// Four waits capped at five seconds is long enough to outlast a burst and short
// enough that a game master waiting on an image is not.
func TestRetriesAreBounded(t *testing.T) {
	r := newRetryer()

	if got := r.MaxAttempts(); got != retryAttempts {
		t.Errorf("MaxAttempts() = %d, want %d", got, retryAttempts)
	}

	for attempt := 1; attempt < retryAttempts; attempt++ {
		delay, err := r.RetryDelay(attempt, throttleResponse(http.StatusTooManyRequests))
		if err != nil {
			t.Fatalf("RetryDelay(%d): %v", attempt, err)
		}
		if delay > retryBackoff {
			t.Errorf("attempt %d waits %s, past the %s cap", attempt, delay, retryBackoff)
		}
	}
}

// throttleResponse is an error shaped the way the SDK's status check reads one:
// it looks for something that can report an HTTP status, which is what the
// transport wraps every response in.
func throttleResponse(status int) error {
	return &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
			Err:      errors.New("throttled"),
		},
	}
}
