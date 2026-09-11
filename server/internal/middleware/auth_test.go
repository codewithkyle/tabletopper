package middleware
import (
	"net/http"
	"net/http/httptest"
	"tabletopper/internal/session"
	"testing"
)
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
func signedInRequest(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Cookie", sessionCookie)
	return r
}
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
func TestOptionalSessionLeavesALoggedOutPageCacheable(t *testing.T) {
	h := signedIn.OptionalSession(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q on a logged-out page, want it unset", got)
	}
}
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
