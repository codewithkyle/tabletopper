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
	render(w, r, pages.SignIn(a.clerkFrontend()))
}

func (a *App) SignUp(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.SignUp(a.clerkFrontend()))
}

// clerkFrontend is the browser-side Clerk configuration. The script is
// clerk-js 4, served from the instance's own frontend API as Clerk documents.
func (a *App) clerkFrontend() pages.ClerkFrontend {
	return pages.ClerkFrontend{
		PublishableKey: a.Config.ClerkPublishableKey,
		ScriptURL:      a.Config.ClerkFrontendAPI + "/npm/@clerk/clerk-js@4/dist/clerk.browser.js",
	}
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
