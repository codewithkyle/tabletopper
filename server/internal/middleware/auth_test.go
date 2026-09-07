package middleware

import (
	"net/http"
	"net/http/httptest"
	"tabletopper/internal/session"
	"testing"
)

// NOTE: a request without a session cookie short-circuits before the store's
// queries are touched, so these cases need no database behind it.
var auth = Auth{Sessions: session.NewStore(nil, false)}

func TestRequireSessionRedirectsWithoutCookie(t *testing.T) {
	called := false
	h := auth.RequireSession(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/characters", nil))

	if called {
		t.Fatal("handler ran without a session")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/sign-in" {
		t.Errorf("Location = %q, want %q", got, "/sign-in")
	}
}

func TestRequireSessionUsesHXRedirectForHTMX(t *testing.T) {
	h := auth.RequireSession(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler ran without a session")
	})

	req := httptest.NewRequest(http.MethodDelete, "/assets/maps/abc", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h(rec, req)

	if got := rec.Header().Get("HX-Redirect"); got != "/sign-in" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/sign-in")
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want empty so htmx handles the navigation", got)
	}
}

func TestRequireSessionOr404WithoutCookie(t *testing.T) {
	h := auth.RequireSessionOr404(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler ran without a session")
	})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/assets/images/abc", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestOptionalSessionContinuesWithoutCookie(t *testing.T) {
	called := false
	h := auth.OptionalSession(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if s := session.FromContext(r.Context()); !s.UserID.IsZero() {
			t.Errorf("UserID = %v, want the zero value for a logged out visitor", s.UserID)
		}
	})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("handler did not run")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// signedInRequest is a GET carrying the cookie the stub in session_db_test.go
// answers.
func signedInRequest(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Cookie", sessionCookie)

	return r
}

// A page rendered for one signed-in reader must not be replayable out of a
// cache after they log out, which on a shared machine is the whole of the
// exposure. The fragments have carried this from the start; the pages they swap
// into did not.
func TestRequireSessionMarksThePageUncacheable(t *testing.T) {
	called := false
	h := signedIn.RequireSession(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	h(rec, signedInRequest("/characters"))

	if !called {
		t.Fatalf("handler did not run; the session stub answered %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestOptionalSessionMarksASignedInPageUncacheable(t *testing.T) {
	called := false
	h := signedIn.OptionalSession(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	h(rec, signedInRequest("/"))

	if !called {
		t.Fatal("handler did not run")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

// The homepage logged out is the same document for everybody and is the one
// page worth a cache having, so OptionalSession must not mark it.
func TestOptionalSessionLeavesALoggedOutPageCacheable(t *testing.T) {
	h := signedIn.OptionalSession(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q on a logged-out page, want it unset", got)
	}
}

// The asset routes set their own policy -- a tile is immutable, an avatar is
// private, no-cache -- and a no-store written here would have been replaced by
// each of them anyway. Leaving it out is deliberate; this is what says so.
func TestRequireSessionOr404LeavesTheCachePolicyToTheHandler(t *testing.T) {
	h := signedIn.RequireSessionOr404(func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	h(rec, signedInRequest("/assets/images/abc"))

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want it unset so the handler decides", got)
	}
}

func TestFragmentsStayUncacheableAndUnindexed(t *testing.T) {
	called := false
	h := signedIn.Fragment(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	h(rec, signedInRequest("/fragment/character/spell-card"))

	if !called {
		t.Fatal("handler did not run")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("X-Robots-Tag = %q, want %q", got, "noindex")
	}
}
