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









func TestDeleteManyWithNoKeysDoesNothing(t *testing.T) {
	c := &Client{bucket: "test"}

	if err := c.DeleteMany(context.Background(), nil); err != nil {
		t.Errorf("DeleteMany(nil) = %v, want nil", err)
	}
	if err := c.DeleteMany(context.Background(), []string{}); err != nil {
		t.Errorf("DeleteMany([]) = %v, want nil", err)
	}
}



func TestDeleteManyRefusesAnEmptyKey(t *testing.T) {
	c := &Client{bucket: "test"}

	if err := c.DeleteMany(context.Background(), []string{"users/a/journals/b", ""}); err == nil {
		t.Error("DeleteMany with an empty key = nil, want an error")
	}
}







func TestDeletePrefixRefusesAnythingThatIsNotADirectory(t *testing.T) {
	c := &Client{bucket: "test"}

	for _, prefix := range []string{"", "users/a/maps/01JB", "users/a/maps/01JB/original"} {
		if err := c.DeletePrefix(context.Background(), prefix); err == nil {
			t.Errorf("DeletePrefix(%q) = nil, want an error", prefix)
		}
	}
}




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





func TestTheSDKsOwnRetryRulesSurvive(t *testing.T) {
	if !newRetryer().IsErrorRetryable(throttleResponse(http.StatusServiceUnavailable)) {
		t.Error("a 503 is no longer retried")
	}
	if newRetryer().IsErrorRetryable(throttleResponse(http.StatusNotFound)) {
		t.Error("a 404 is retried")
	}
}





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




func throttleResponse(status int) error {
	return &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
			Err:      errors.New("throttled"),
		},
	}
}
