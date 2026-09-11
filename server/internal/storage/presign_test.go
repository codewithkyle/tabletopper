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





func testClient(t *testing.T) *Client {
	t.Helper()

	cfg := aws.Config{
		Region:      "auto",
		Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "secrettest", ""),
	}
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https:
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
	
	
	
	if headers := signedHeaders(t, signed); len(headers) != 1 || headers[0] != "host" {
		t.Errorf("signed headers = %v, want just host -- a range request sends headers this URL cannot know about", headers)
	}
}



func TestPresigningRefusesAnEmptyKey(t *testing.T) {
	client := testClient(t)

	if _, err := client.PresignPut(context.Background(), "", "audio/mpeg", 1, time.Hour); err == nil {
		t.Error("PresignPut signed an empty key")
	}
	if _, err := client.PresignGet(context.Background(), "", time.Hour); err == nil {
		t.Error("PresignGet signed an empty key")
	}
}



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
