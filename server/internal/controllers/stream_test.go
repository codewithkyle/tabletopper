package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type objectGetter func(context.Context, string) (io.ReadCloser, int64, error)

func TestAnImageCarriesItsCachePolicyOnlyWhenItIsServed(t *testing.T) {
	const etag = `"01BX5ZZKBKACTAV9WEVGEMMVS3-0-0-0"`
	served := func(context.Context, string) (io.ReadCloser, int64, error) {
		return io.NopCloser(strings.NewReader("webp")), 4, nil
	}
	failed := func(context.Context, string) (io.ReadCloser, int64, error) {
		return nil, 0, errors.New("the bucket is throttling")
	}
	abandoned := func(ctx context.Context, _ string) (io.ReadCloser, int64, error) {
		return nil, 0, ctx.Err()
	}
	for name, c := range map[string]struct {
		get    objectGetter
		match  string
		cancel bool
		status int
		body   string
		cached bool
	}{
		"an object that is served":              {get: served, status: http.StatusOK, body: "webp", cached: true},
		"a revalidation that matches":           {get: failed, match: etag, status: http.StatusNotModified, cached: true},
		"a revalidation that does not match":    {get: served, match: `"stale"`, status: http.StatusOK, body: "webp", cached: true},
		"an object the bucket will not return":  {get: failed, status: http.StatusNotFound},
		"a request the client walked away from": {get: abandoned, cancel: true, status: http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/assets/maps/tile.webp", nil)
			if c.match != "" {
				r.Header.Set("If-None-Match", c.match)
			}
			if c.cancel {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			streamObject(rec, r, c.get, "maps/tile.webp", etag, cacheImmutable)
			if rec.Code != c.status {
				t.Errorf("status = %d, want %d", rec.Code, c.status)
			}
			if body := rec.Body.String(); body != c.body {
				t.Errorf("body = %q, want %q", body, c.body)
			}
			cache, tag := rec.Header().Get("Cache-Control"), rec.Header().Get("ETag")
			if c.cached {
				if cache != cacheImmutable {
					t.Errorf("Cache-Control = %q, want %q", cache, cacheImmutable)
				}
				if tag != etag {
					t.Errorf("ETag = %q, want %q", tag, etag)
				}
				return
			}
			if cache != "" {
				t.Errorf("Cache-Control = %q on a response that is not the image, want nothing", cache)
			}
			if tag != "" {
				t.Errorf("ETag = %q on a response that is not the image, want nothing", tag)
			}
		})
	}
}
func TestAServedImageSaysWhatItIsAndHowLongItIs(t *testing.T) {
	rec := httptest.NewRecorder()
	streamObject(rec, httptest.NewRequest(http.MethodGet, "/assets/images/x", nil),
		func(context.Context, string) (io.ReadCloser, int64, error) {
			return io.NopCloser(strings.NewReader("webp")), 4, nil
		},
		"images/x.webp", `"x"`, cacheRevalidate,
	)
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Errorf("Content-Type = %q, want image/webp", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "4" {
		t.Errorf("Content-Length = %q, want 4", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != cacheRevalidate {
		t.Errorf("Cache-Control = %q, want the policy the caller passed", got)
	}
}
