package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFragmentRedirectsWithoutTheHTMXHeader(t *testing.T) {
	h := auth.Fragment(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler ran without a session")
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/fragment/character/info-row", nil))
	if got := rec.Header().Get("HX-Redirect"); got != "/sign-in" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/sign-in")
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want empty so htmx handles the navigation", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d so htmx reads the header rather than swapping", rec.Code, http.StatusOK)
	}
}
func TestFragmentSetsNoStoreAndNoIndex(t *testing.T) {
	h := auth.Fragment(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/fragment/character/feature-row", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("X-Robots-Tag = %q, want %q", got, "noindex")
	}
}
func TestFragmentNotFoundIsEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	FragmentNotFound(rec, httptest.NewRequest(http.MethodGet, "/fragment/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}
