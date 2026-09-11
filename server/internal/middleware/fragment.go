package middleware
import (
	"log/slog"
	"net/http"
	"tabletopper/internal/htmx"
)
func (m Auth) Fragment(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex")
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			htmx.Redirect(w, "/sign-in")
			return
		}
		m.refresh(w, r, &s)
		next(w, withSession(r, s))
	}
}
func FragmentNotFound(w http.ResponseWriter, r *http.Request) {
	slog.Warn("404 Not Found (fragment)", "path", r.URL.Path)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusNotFound)
}
