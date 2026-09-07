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
	"strconv"
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
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
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

	return &Client{
		s3:      s3Client,
		presign: s3.NewPresignClient(s3Client),
		bucket:  cfg.Bucket,
	}, nil
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

// PutReader writes body to key, overwriting whatever was there.
//
// IT IS THE ONE UPLOAD THAT NEVER HOLDS WHAT IT IS SENDING. Everything else in
// here is handing over a WebP this process just encoded, which is already in
// memory and is kilobytes; a map's original is up to 128 MiB that arrived over
// the wire, and ParseMultipartForm has already spilled it to a temp file. So
// the map path passes the multipart.File straight through, and nothing copies
// it back into the heap to get here.
//
// body MUST BE SEEKABLE, which is why the parameter is an io.ReadSeeker and not
// an io.Reader: the S3 client reads it once to checksum and sign, and again to
// send, and it rewinds between the two. Both forms of multipart.File satisfy
// that -- an *os.File when the part spilled, a section reader when it did not
// -- so the client treats either exactly as it treats a bytes.Reader.
//
// size is passed because it is known. Without a ContentLength the client has to
// find the length itself, which for a seekable body means seeking to the end
// and back, and for anything else means buffering.
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

// Put writes body to key, overwriting whatever was there. It is PutReader for
// the callers that already hold their bytes -- every encoded image, and every
// tile.
func (c *Client) Put(ctx context.Context, key string, body []byte, contentType string) error {
	return c.PutReader(ctx, key, bytes.NewReader(body), int64(len(body)), contentType)
}

// PRESIGNING, WHICH IS HOW MUSIC GETS IN AND OUT WITHOUT TOUCHING THIS PROCESS.
//
// A track is one to two hours long and 115 to 175 MB. Every other upload here is
// a multipart body this server reads into memory and forwards; at that size
// that is a 175 MB allocation, a spill to the container's /tmp, and the same
// bytes crossing the network twice. A presigned URL is a signature over a
// request the browser then makes itself, so the bytes go browser-to-bucket and
// the server exchanges a few hundred bytes of URL.
//
// NOTHING HERE TOUCHES THE NETWORK. Presigning is an HMAC over a canonical
// request; it needs credentials and no connection, and it will happily sign a
// URL for a key that does not exist.

// PresignPut returns a URL the browser may PUT exactly `size` bytes of exactly
// `contentType` to, until it expires.
//
// BOTH ARE SIGNED, NOT SUGGESTED, and that is the whole reason this takes them.
// Content-Length is in the signature, so R2 refuses a body that is not the
// length the URL was minted for -- which is what keeps an upload cap
// enforceable when the bytes never pass through a handler that could measure
// them. Content-Type is in the signature for a smaller reason: R2 stores what it
// is given and serves it back on the GET, so this is where a track's type is
// decided for good.
//
// The browser sends Content-Length itself for a body of known size, so the only
// header a caller has to set by hand is Content-Type.
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

// PresignGet returns a URL the browser may GET the object from until it expires.
//
// R2 SERVES RANGES ON IT, which is the point: an <audio> element seeks by asking
// for byte ranges, and Safari will not play a source that cannot answer one.
// Proxying the bytes through this process would mean implementing 206 and
// Content-Range here, and streaming every player's copy of a 175 MB track
// through the Go server for the length of a session.
//
// A URL is checked when the request is made and not while the response streams,
// so an expiry shorter than the track is not a problem -- but every player is
// re-minted on demand by the redirect route anyway, so nothing here has to
// outlive one request.
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

// ErrNotFound is the object not being there, told apart from every other way a
// request to R2 can fail.
//
// THE DIFFERENCE IS WORTH A SENTINEL BECAUSE ONE CALLER DELETES ON IT. A music
// confirm follows a ten-minute, 175 MB PUT; "the object never arrived" means
// the upload failed and the row is rubbish, while a 5xx from R2, a timeout or a
// dropped connection means the object is very probably sitting there and the
// only thing that failed is the question. Without the distinction both answers
// threw the upload away, and the second one is the common one.
var ErrNotFound = errors.New("storage: object not found")

// Size reports how many bytes an object holds, and is the cheapest question
// that can be asked about one -- a HEAD, which transfers none of it.
//
// It is what a confirm asks first: an object that is not there at all is an
// upload that never finished, and one larger than the cap is a signature that
// was not honoured. A missing object is ErrNotFound and nothing else is.
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

// notFound maps the two ways the S3 API says "no such object" onto ErrNotFound
// and leaves every other error as it was.
//
// THERE ARE TWO BECAUSE HEAD AND GET DISAGREE. A HeadObject has no body to put
// an error code in, so the SDK models its 404 as *types.NotFound; a GetObject
// gets the code and models it as *types.NoSuchKey. A caller that only checked
// one would work on Size and silently fall through on Peek.
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

// Peek reads the first n bytes of an object and nothing more.
//
// IT IS A RANGED GET, so identifying a 175 MB track costs 64 bytes off the wire.
// The Range header is what makes that true; without it this would download the
// whole object to look at its first line.
//
// A short object answers with what it has rather than an error, so a caller gets
// fewer bytes than it asked for and has to cope -- which audio.TypeForBytes
// does, by checking the length of every signature it tests.
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
