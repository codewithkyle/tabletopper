// Package storage is the object store behind every uploaded image: a
// Cloudflare R2 bucket spoken to over the S3 API. One Client is built at
// startup and shared; it is safe for concurrent use.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

type Client struct {
	s3     *s3.Client
	bucket string
}

// New builds the shared client. It does not touch the network: a bad
// credential surfaces on the first request, the same as before, but a
// missing one is refused here.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, errors.New("storage: incomplete R2 configuration")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: aws config: %w", err)
	}

	endpoint := "https://" + cfg.AccountID + ".r2.cloudflarestorage.com"
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		o.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	})

	return &Client{s3: s3Client, bucket: cfg.Bucket}, nil
}

// Delete removes one object. Deleting a key that does not exist succeeds.
func (c *Client) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("storage: empty key")
	}

	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

// deleteBatchSize is how many keys go in one DeleteObjects call. It is the S3
// limit rather than a tuning choice.
const deleteBatchSize = 1000

// DeleteMany removes any number of objects, a thousand at a time. An empty list
// is a no-op with no network round trip.
//
// A KEY THAT COULD NOT BE DELETED COMES BACK INSIDE A 200, in the response's
// Errors rather than as an error, so the response is checked and the first
// failure is returned. The caller is expected to keep every row in the batch
// and retry the whole batch, which is safe because deleting a key that is
// already gone succeeds.
func (c *Client) DeleteMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	objects := make([]types.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			return errors.New("storage: empty key")
		}
		objects = append(objects, types.ObjectIdentifier{Key: aws.String(key)})
	}

	for start := 0; start < len(objects); start += deleteBatchSize {
		out, err := c.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.bucket),
			// Quiet, so the reply carries the failures and nothing else --
			// a thousand successful deletes have nothing to report.
			Delete: &types.Delete{
				Objects: objects[start:min(start+deleteBatchSize, len(objects))],
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			failed := out.Errors[0]
			return fmt.Errorf("storage: delete %q: %s", aws.ToString(failed.Key), aws.ToString(failed.Message))
		}
	}

	return nil
}

// DeletePrefix removes every object under a prefix, a page at a time. It is
// how a whole tile pyramid goes: one list and one batch delete per thousand
// objects rather than a call per tile.
//
// THE PREFIX MUST END IN "/". Without that rule a prefix is a string match
// rather than a directory, so a caller one character short of the boundary
// takes a sibling with it -- and the empty prefix, which is what a
// zero-valued id builds, takes the entire bucket. Every prefix this store
// hands out is a directory, so requiring it costs nothing and the mistake
// cannot be made.
//
// ListObjectsV2 pages at a thousand keys and DeleteMany batches at a thousand,
// so each page is exactly one delete call. A failure partway through leaves
// what has not been reached yet; the caller retries the whole prefix, which is
// safe because deleting a key that is already gone succeeds.
func (c *Client) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" || !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("storage: %q is not a prefix", prefix)
	}

	pages := s3.NewListObjectsV2Paginator(c.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}

		keys := make([]string, 0, len(page.Contents))
		for _, object := range page.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
		if err := c.DeleteMany(ctx, keys); err != nil {
			return err
		}
	}

	return nil
}

// Get opens one object for reading. The caller owns the body and must close
// it. size is the object's length, or -1 when R2 did not say.
func (c *Client) Get(ctx context.Context, key string) (body io.ReadCloser, size int64, err error) {
	if key == "" {
		return nil, -1, errors.New("storage: empty key")
	}

	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, -1, err
	}

	size = -1
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, nil
}

// Put writes body to key, overwriting whatever was there.
func (c *Client) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if key == "" {
		return errors.New("storage: empty key")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}
