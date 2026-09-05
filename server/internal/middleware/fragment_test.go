package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// NOTE: as in auth_test.go, a request with no session cookie short-circuits
// before the DB pool is touched, so none of these need a database.

// The point of Fragment over RequireSession: RequireSession picks its redirect
// shape from the HX-Request header, and this one does not have to, because the
// prefix already established that the caller is a swap. A fragment route asked
// for a session it does not have answers with HX-Redirect whether or not the
// header is there -- a 303 would be followed by fetch() and the sign-in page
// swapped into whatever target the caller named.
func TestFragmentRedirectsWithoutTheHTMXHeader(t *testing.T) {
	h := Fragment(func(w http.ResponseWriter, r *http.Request) {
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

// Both headers are set before the session is looked at, so they are on the
// response even when the fragment is never rendered.
func TestFragmentSetsNoStoreAndNoIndex(t *testing.T) {
	h := Fragment(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/fragment/character/spell-card", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("X-Robots-Tag = %q, want %q", got, "noindex")
	}
}

// The body stays empty so noSwap leaves the caller's target alone; anything
// page-shaped here would be swapped into a <div> if the config ever changed.
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
