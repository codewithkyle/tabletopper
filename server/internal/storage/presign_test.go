package storage

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// testClient is a Client with credentials that are not real and an endpoint that
// does not exist. That is enough for every test here, because PRESIGNING NEVER
// TOUCHES THE NETWORK: it is an HMAC over a canonical request, and it will
// happily sign a URL for a key in a bucket that was never created.
func testClient(t *testing.T) *Client {
	t.Helper()

	cfg := aws.Config{
		Region:      "auto",
		Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "secrettest", ""),
	}
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://account.r2.cloudflarestorage.com")
		o.UsePathStyle = true
		o.HTTPClient = &http.Client{}
	})

	return &Client{s3: s3Client, presign: s3.NewPresignClient(s3Client), bucket: "tabletopper"}
}

func signedHeaders(t *testing.T, signed string) []string {
	t.Helper()

	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("the signed URL does not parse: %v", err)
	}

	return strings.Split(u.Query().Get("X-Amz-SignedHeaders"), ";")
}

// THE CAP ON A MUSIC UPLOAD IS THIS SIGNATURE AND NOTHING ELSE.
//
// The bytes never reach a handler that could count them -- that is the whole
// point of presigning the PUT -- so the size the browser declares would be worth
// nothing if it were only checked in Go. Signing it as Content-Length is what
// makes R2 refuse a body that is not exactly that long, which turns a number a
// caller chose into a promise the bucket enforces.
//
// If this ever stops holding, the failure is silent and total: uploads keep
// working, and the 256 MiB cap becomes a suggestion.
func TestAPresignedPutBindsTheSizeAndTypeItWasMintedFor(t *testing.T) {
	signed, err := testClient(t).PresignPut(context.Background(), "users/u/music/t", "audio/mpeg", 1234, time.Hour)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}

	headers := signedHeaders(t, signed)
	for _, want := range []string{"content-length", "content-type", "host"} {
		var found bool
		for _, got := range headers {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is not signed into the URL (signed: %v)", want, headers)
		}
	}

	// A CHECKSUM HEADER WOULD BREAK THE UPLOAD OUTRIGHT, so its absence is
	// asserted rather than assumed. Recent versions of this SDK add
	// x-amz-checksum-* to PutObject by default; signed into a presigned URL,
	// that would demand a header a browser has no way to compute, and every
	// PUT would come back 403 with nothing in the app to explain it.
	for _, got := range headers {
		if strings.HasPrefix(got, "x-amz-checksum") || got == "x-amz-sdk-checksum-algorithm" {
			t.Errorf("the SDK signed %q, which a browser cannot send", got)
		}
	}
}

func TestAPresignedGetIsAPlainReadableURL(t *testing.T) {
	signed, err := testClient(t).PresignGet(context.Background(), "users/u/music/t", time.Hour)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}

	if !strings.Contains(signed, "X-Amz-Signature=") {
		t.Errorf("the URL carries no signature: %s", signed)
	}
	// Nothing but the host is signed, so the browser sends whatever headers it
	// likes -- which is what lets an <audio> element add a Range and still be
	// served.
	if headers := signedHeaders(t, signed); len(headers) != 1 || headers[0] != "host" {
		t.Errorf("signed headers = %v, want just host -- a range request sends headers this URL cannot know about", headers)
	}
}

// An empty key is a bug upstream -- a row with no file_path -- and signing one
// would produce a URL pointing at the bucket root.
func TestPresigningRefusesAnEmptyKey(t *testing.T) {
	client := testClient(t)

	if _, err := client.PresignPut(context.Background(), "", "audio/mpeg", 1, time.Hour); err == nil {
		t.Error("PresignPut signed an empty key")
	}
	if _, err := client.PresignGet(context.Background(), "", time.Hour); err == nil {
		t.Error("PresignGet signed an empty key")
	}
}

// The expiry is in the URL, so a stale one is refused by R2 rather than by
// anything here.
func TestAPresignedURLCarriesItsExpiry(t *testing.T) {
	signed, err := testClient(t).PresignPut(context.Background(), "users/u/music/t", "audio/mpeg", 1, 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}

	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := u.Query().Get("X-Amz-Expires"); got != "900" {
		t.Errorf("X-Amz-Expires = %q, want 900", got)
	}
}
