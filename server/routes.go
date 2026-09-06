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
	// The editor is a page per tab, and twelve of them, because the spells tab
	// is one page per level. Autosave is what makes them pages rather than
	// sections of one: nothing is ever held unsaved, so moving between them
	// loses nothing and each can be linked and reloaded.
	//
	// The bare /edit/spells is not one of them. The tab points straight at
	// cantrips and there is no index above the levels; that route is a redirect
	// so a bookmark to it still lands somewhere.
	//
	// {level} is bounded to 0-9 before it reaches a query, and a level outside
	// that is a redirect to cantrips rather than a 404 -- the character is real
	// and only the last segment is wrong.
	mux.HandleFunc("GET /characters/{id}/edit", auth.RequireSession(app.CharacterPage))
	mux.HandleFunc("GET /characters/{id}/edit/inventory", auth.RequireSession(app.CharacterInventoryPage))
	mux.HandleFunc("GET /characters/{id}/edit/spells", auth.RequireSession(app.CharacterSpellsRedirect))
	mux.HandleFunc("GET /characters/{id}/edit/spells/{level}", auth.RequireSession(app.CharacterSpellLevelPage))
	mux.HandleFunc("DELETE /characters/{id}", auth.RequireSession(app.DeleteCharacter))
	mux.HandleFunc("POST /characters/{id}/avatar", auth.RequireSession(app.UploadCharacterAvatar))

	// The character editor autosaves a panel at a time. Each of these owns a
	// disjoint set of columns and writes only those; none of them shares a
	// handler or a query with a statement wide enough to write the whole sheet,
	// which would fill the absent columns with defaults.
	//
	// Mutations, so they keep resource URLs and stay off /fragment/ -- the
	// prefix names a representation, and none of these returns one. The reply is
	// a toast and the panel's error block rendered empty, which is what clears a
	// message the previous save left there.
	//
	// Only the bonuses route takes its panel name from the path, because the
	// skills and saving-throw grids differ in a field prefix and a column and in
	// nothing else. The handler matches that segment against an allowlist before
	// it reaches a query. The repeaters were the same shape until inventory and
	// spells replaced three of the four, and a parameter with one legal value is
	// worse than no parameter -- so Features is a route that names itself.
	mux.HandleFunc("POST /characters/{id}/identity", auth.RequireSession(app.SaveCharacterIdentity))
	mux.HandleFunc("POST /characters/{id}/abilities", auth.RequireSession(app.SaveCharacterAbilities))
	mux.HandleFunc("POST /characters/{id}/core-stats", auth.RequireSession(app.SaveCharacterCoreStats))
	mux.HandleFunc("POST /characters/{id}/proficiencies", auth.RequireSession(app.SaveCharacterProficiencies))
	mux.HandleFunc("POST /characters/{id}/features", auth.RequireSession(app.SaveCharacterFeatures))
	mux.HandleFunc("POST /characters/{id}/bonuses/{kind}", auth.RequireSession(app.SaveCharacterBonuses))

	// Inventory is the one part of the sheet where the ROW is the unit of work
	// rather than the panel, so it gets a collection and a member rather than a
	// single save. Its rows are read back by the Character tab, and a row
	// referenced from a second view needs an identity that survives an edit --
	// which the panels' whole-list rewrite could not give it.
	//
	// The add answers with the row it created. That is the mutation case the
	// fragment rules name: the prefix marks GET-shaped representations, and the
	// alternative here is a POST that returns nothing followed by a GET to fetch
	// what it just made.
	//
	// The delete answers 200 and not 204 -- noSwap lists 204, and a status in
	// that list overrides hx-swap="delete", which would leave the row on screen
	// after the row was gone.
	mux.HandleFunc("POST /characters/{id}/inventory", auth.RequireSession(app.AddInventoryItem))
	mux.HandleFunc("POST /characters/{id}/inventory/{itemId}", auth.RequireSession(app.SaveInventoryItem))
	mux.HandleFunc("DELETE /characters/{id}/inventory/{itemId}", auth.RequireSession(app.DeleteInventoryItem))

	// Spells are the same shape as inventory -- a collection and a member per
	// row -- with the level carried in the path. It is there because a spell
	// cannot change level, so it identifies the row as much as the id does, and
	// because /spells/{spellId} and /spells/{level} are the same pattern to the
	// mux and one of them had to grow a segment.
	//
	// slots is a literal in the first position, so /spells/slots/{level} matches
	// a strict subset of /spells/{level}/{spellId} and the mux takes the more
	// specific of the two without a conflict. No level is ever the word slots:
	// parseSpellLevel only returns for 0 through 9.
	//
	// The counters are their own route rather than a field on the level page,
	// because the overview renders all ten and each is its own form. One handler
	// serves both pages, and a save writes one row.
	mux.HandleFunc("POST /characters/{id}/spells/slots/{level}", auth.RequireSession(app.SaveSpellSlots))
	mux.HandleFunc("POST /characters/{id}/spells/{level}", auth.RequireSession(app.AddSpell))
	mux.HandleFunc("POST /characters/{id}/spells/{level}/{spellId}", auth.RequireSession(app.SaveSpell))
	mux.HandleFunc("DELETE /characters/{id}/spells/{level}/{spellId}", auth.RequireSession(app.DeleteSpell))

	// The journal is a page per entry plus the list, and its mutations are the
	// collection-and-member pair inventory and spells already use.
	//
	// CREATE IS A PLAIN FORM POST. It collects nothing -- an entry is born blank
	// and titled in the editor -- so there is no field to reject and no state to
	// keep, and the handler answers with a 303 the browser follows into the new
	// entry's page. Every other route here is htmx.
	//
	// The delete answers 200 and not 204, for the reason the inventory block
	// above gives: noSwap lists 204, and a status in that list overrides the
	// hx-swap="delete" on the button.
	mux.HandleFunc("GET /characters/{id}/edit/journal", auth.RequireSession(app.CharacterJournalPage))
	mux.HandleFunc("GET /characters/{id}/edit/journal/{entryId}", auth.RequireSession(app.CharacterJournalEntryPage))
	mux.HandleFunc("POST /characters/{id}/journal", auth.RequireSession(app.CreateJournalEntry))
	mux.HandleFunc("POST /characters/{id}/journal/{entryId}", auth.RequireSession(app.SaveJournalEntry))
	mux.HandleFunc("DELETE /characters/{id}/journal/{entryId}", auth.RequireSession(app.DeleteJournalEntry))

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
	mux.HandleFunc("GET /fragment/character/feature-row", auth.Fragment(app.FeatureRowFragment))
	mux.HandleFunc("GET /fragment/character/journal-link", auth.Fragment(app.JournalLinkFragment))

	// Subtree pattern, so it takes any /fragment/ path the three above did not.
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
