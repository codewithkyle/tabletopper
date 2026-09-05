package controllers

import (
	"net/http"

	"tabletopper/internal/session"
	"tabletopper/templ/pages"
)

// Homepage renders both the logged-out landing and the logged-in menu; the
// page decides which from the session, which may be the zero value.
func (a *App) Homepage(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Homepage(session.FromContext(r.Context())))
}

func (a *App) SignIn(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.SignIn())
}

func (a *App) SignUp(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.SignUp())
}

func (a *App) TOS(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.TOS())
}

func (a *App) PrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.PrivacyPolicy())
}

func (a *App) ServerError(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.ServerError())
}
