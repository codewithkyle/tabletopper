package storage
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
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
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
}
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, errors.New("storage: incomplete R2 configuration")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsconfig.WithRetryer(newRetryer),
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
	return &Client{
		s3:      s3Client,
		presign: s3.NewPresignClient(s3Client),
		bucket:  cfg.Bucket,
	}, nil
}
const (
	retryAttempts = 5
	retryBackoff = 5 * time.Second
)
var (
	throttleStatus = map[int]struct{}{http.StatusTooManyRequests: {}}
	throttleCode   = map[string]struct{}{"ServiceUnavailable": {}}
)
func newRetryer() aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = retryAttempts
		o.MaxBackoff = retryBackoff
		o.Retryables = append(o.Retryables,
			retry.RetryableHTTPStatusCode{Codes: throttleStatus},
			retry.RetryableErrorCode{Codes: throttleCode},
		)
	})
}
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
const deleteBatchSize = 1000
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
func (c *Client) PutReader(ctx context.Context, key string, body io.ReadSeeker, size int64, contentType string) error {
	if key == "" {
		return errors.New("storage: empty key")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}
func (c *Client) Put(ctx context.Context, key string, body []byte, contentType string) error {
	return c.PutReader(ctx, key, bytes.NewReader(body), int64(len(body)), contentType)
}
func (c *Client) PresignPut(ctx context.Context, key string, contentType string, size int64, ttl time.Duration) (string, error) {
	if key == "" {
		return "", errors.New("storage: empty key")
	}
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if key == "" {
		return "", errors.New("storage: empty key")
	}
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
var ErrNotFound = errors.New("storage: object not found")
func (c *Client) Size(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return 0, errors.New("storage: empty key")
	}
	head, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, notFound(err)
	}
	if head.ContentLength == nil {
		return 0, errors.New("storage: object has no length")
	}
	return *head.ContentLength, nil
}
func notFound(err error) error {
	var missing *types.NotFound
	if errors.As(err, &missing) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return err
}
func (c *Client) Peek(ctx context.Context, key string, n int64) ([]byte, error) {
	if key == "" {
		return nil, errors.New("storage: empty key")
	}
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-" + strconv.FormatInt(n-1, 10)),
	})
	if err != nil {
		return nil, notFound(err)
	}
	defer out.Body.Close()
	return io.ReadAll(io.LimitReader(out.Body, n))
}
