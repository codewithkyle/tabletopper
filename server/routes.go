package main

import (
	"log/slog"
	"net/http"

	"tabletopper/internal/controllers"
	"tabletopper/internal/middleware"
)

// routes is the whole URL space, in one place. Every pattern names a method:
// a method-less pattern would answer a POST to a page route with the page.
func routes(app *controllers.App, auth middleware.Auth) http.Handler {
	mux := http.NewServeMux()

	// "/{$}" is exactly the root; the bare "/" below is the catch-all that
	// logs what missed.
	mux.HandleFunc("GET /{$}", auth.OptionalSession(app.Homepage))
	mux.HandleFunc("/", notFound)

	mux.HandleFunc("GET /sign-in", app.SignIn)
	mux.HandleFunc("GET /sign-up", app.SignUp)
	mux.HandleFunc("GET /authorize", app.Authorize)
	mux.HandleFunc("GET /logout", app.Logout)

	mux.HandleFunc("GET /tos", app.TOS)
	mux.HandleFunc("GET /privacy", app.PrivacyPolicy)
	mux.HandleFunc("GET /error", app.ServerError)

	mux.HandleFunc("GET /characters", auth.RequireSession(app.CharactersPage))
	// Creation has no page. It is a dialog on the characters page carrying one
	// field, served by the fragment route below; this takes the name it collects
	// and redirects to the editor, which saves the rest as it is filled in.
	mux.HandleFunc("POST /characters", auth.RequireSession(app.NewCharacterForm))
	// The editor is two pages, one per tab. Autosave is what makes them pages
	// rather than sections of one: nothing is ever held unsaved, so moving
	// between them loses nothing and each can be linked and reloaded.
	mux.HandleFunc("GET /characters/{id}/edit", auth.RequireSession(app.CharacterPage))
	mux.HandleFunc("GET /characters/{id}/edit/spells", auth.RequireSession(app.CharacterSpellsPage))
	mux.HandleFunc("DELETE /characters/{id}", auth.RequireSession(app.DeleteCharacter))
	mux.HandleFunc("POST /characters/{id}/avatar", auth.RequireSession(app.UploadCharacterAvatar))

	// The character editor autosaves a panel at a time. Each of these owns a
	// disjoint set of columns and writes only those; none of them shares a
	// handler or a query with POST /characters/{id}, which writes all 31 and
	// would fill the absent ones with defaults.
	//
	// Mutations, so they keep resource URLs and stay off /fragment/ -- the
	// prefix names a representation, and none of these returns one. The reply
	// is a toast and a 204.
	//
	// The last two take the panel name from the path because the grids and the
	// repeaters differ only in a field prefix and a column. Both handlers match
	// that segment against an allowlist before it reaches a query.
	mux.HandleFunc("POST /characters/{id}/identity", auth.RequireSession(app.SaveCharacterIdentity))
	mux.HandleFunc("POST /characters/{id}/abilities", auth.RequireSession(app.SaveCharacterAbilities))
	mux.HandleFunc("POST /characters/{id}/core-stats", auth.RequireSession(app.SaveCharacterCoreStats))
	mux.HandleFunc("POST /characters/{id}/proficiencies", auth.RequireSession(app.SaveCharacterProficiencies))
	mux.HandleFunc("POST /characters/{id}/spells", auth.RequireSession(app.SaveCharacterSpells))
	mux.HandleFunc("POST /characters/{id}/bonuses/{kind}", auth.RequireSession(app.SaveCharacterBonuses))
	mux.HandleFunc("POST /characters/{id}/rows/{field}", auth.RequireSession(app.SaveCharacterRows))

	mux.HandleFunc("GET /assets", auth.RequireSession(app.AssetsPage))
	mux.HandleFunc("GET /assets/maps", auth.RequireSession(app.MapAssetsPage))
	mux.HandleFunc("POST /assets/maps", auth.RequireSession(app.UploadMap))
	mux.HandleFunc("DELETE /assets/maps/{id}", auth.RequireSession(app.DeleteMap))
	mux.HandleFunc("POST /assets/maps/{id}", auth.RequireSession(app.ReplaceMap))
	mux.HandleFunc("PATCH /assets/maps/{id}/name", auth.RequireSession(app.ReplaceMapName))

	mux.HandleFunc("GET /assets/images/{id}", auth.RequireSessionOr404(app.GetImage))
	mux.HandleFunc("GET /assets/images/{id}/preview", auth.RequireSessionOr404(app.GetImagePreview))

	// Every route below returns partial HTML for a swap into a page that is
	// already open, and the prefix is the only thing that says so. Nothing else
	// does: an hx-get attribute is visible at the call site but not here, and a
	// handler returning a <div> looks exactly like one returning a <html>.
	//
	// The rule is deliberately narrow -- a /fragment/ route is a GET that
	// returns partial HTML, and nothing else. Mutations keep their resource
	// URLs, because POST /fragment/characters would claim the created character
	// lives under /fragment when the path names the resource and the prefix only
	// names the representation. It is also a GET-shaped problem to begin with:
	// only a GET gets bookmarked, linked, crawled or typed into an address bar,
	// which is where confusing a fragment for a page actually costs something.
	//
	// middleware.Fragment carries the contract that follows from that; see it
	// for what a fragment owes its caller.
	mux.HandleFunc("GET /fragment/character/new", auth.Fragment(app.NewCharacterFragment))
	mux.HandleFunc("GET /fragment/character/info-row", auth.Fragment(app.InfoRowFragment))
	mux.HandleFunc("GET /fragment/character/spell-card", auth.Fragment(app.SpellCardFragment))

	// Subtree pattern, so it takes any /fragment/ path the two above did not.
	// Without it these fall to the catch-all on "/" and answer with Go's
	// plain-text 404 page, which is a page-shaped reply to a fragment request.
	mux.HandleFunc("/fragment/", middleware.FragmentNotFound)

	// Static files. The URL prefix is the directory under public/, so one
	// FileServer rooted there covers all four without a StripPrefix each.
	static := http.FileServer(http.Dir("./public"))
	for _, prefix := range []string{"/css/", "/js/", "/static/", "/images/"} {
		mux.Handle("GET "+prefix, static)
	}

	return mux
}

func notFound(w http.ResponseWriter, r *http.Request) {
	slog.Warn("404 Not Found", "path", r.URL.Path)
	http.NotFound(w, r)
}
