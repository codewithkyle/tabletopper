


package middleware

import (
	"log/slog"
	"net/http"

	"tabletopper/internal/htmx"
	"tabletopper/internal/session"
)



type Auth struct {
	Sessions *session.Store
}



func (m Auth) RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			redirectToSignIn(w, r)
			return
		}
		noStore(w)
		m.refresh(w, r, &s)
		next(w, withSession(r, s))
	}
}












func (m Auth) RequireSessionOr404(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		next(w, withSession(r, s))
	}
}



func (m Auth) OptionalSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			next(w, r)
			return
		}
		noStore(w)
		m.refresh(w, r, &s)
		next(w, withSession(r, s))
	}
}














func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}





func (m Auth) refresh(w http.ResponseWriter, r *http.Request, s *session.UserSession) {
	if err := m.Sessions.Refresh(r.Context(), w, s); err != nil {
		slog.Error("Failed to refresh session", "error", err)
	}
}

func withSession(r *http.Request, s session.UserSession) *http.Request {
	return r.WithContext(session.NewContext(r.Context(), s))
}



func redirectToSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		htmx.Redirect(w, "/sign-in")
		return
	}
	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}
