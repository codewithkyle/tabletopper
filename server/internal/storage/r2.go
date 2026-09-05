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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

// Get reads one object into memory.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, errors.New("storage: empty key")
	}

	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
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
