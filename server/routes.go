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
	//
	// Searching the list is a GET returning part of this page, so it is not here
	// -- it is the journal-entries fragment further down.
	mux.HandleFunc("GET /characters/{id}/edit/journal", auth.RequireSession(app.CharacterJournalPage))
	mux.HandleFunc("GET /characters/{id}/edit/journal/{entryId}", auth.RequireSession(app.CharacterJournalEntryPage))
	mux.HandleFunc("POST /characters/{id}/journal", auth.RequireSession(app.CreateJournalEntry))
	mux.HandleFunc("POST /characters/{id}/journal/{entryId}", auth.RequireSession(app.SaveJournalEntry))
	mux.HandleFunc("DELETE /characters/{id}/journal/{entryId}", auth.RequireSession(app.DeleteJournalEntry))

	// An entry's images: a sub-collection of the member above, and the only
	// image pair in the app that is scoped to something narrower than the
	// account. Both carry the character and the entry so the serve route can
	// check every id against the row rather than trust it.
	//
	// THE UPLOAD IS NOT AN HTMX ROUTE. It answers the editor's own fetch with a
	// 201 and a Location header, because its caller is inserting a node in the
	// document rather than swapping markup -- see internal/controllers/
	// journal-images.go. It is still a mutation with a resource URL, so it is
	// here and not under /fragment/.
	//
	// The serve route is RequireSessionOr404 like the two under /assets/images,
	// for the same reason: a redirect to the sign-in page renders as a broken
	// image rather than as a sign-in page.
	mux.HandleFunc("POST /characters/{id}/journal/{entryId}/images", auth.RequireSession(app.UploadJournalImage))
	mux.HandleFunc("GET /characters/{id}/journal/{entryId}/images/{assetId}", auth.RequireSessionOr404(app.GetJournalImage))

	// Sharing one entry: the owner's two mutations here, and the reader's four
	// routes below.
	//
	// The create and the revoke keep the entry's resource URL and stay off
	// /fragment/, like every other mutation. Both answer with the share dialog
	// in whatever state the entry is now in, which is the exception the
	// fragment rules name -- a mutation replying with the thing it just made or
	// changed. The GET that opens that dialog is a fragment and is registered
	// with the rest of them further down.
	mux.HandleFunc("POST /characters/{id}/journal/{entryId}/share", auth.RequireSession(app.CreateJournalShare))
	mux.HandleFunc("DELETE /characters/{id}/journal/{entryId}/share", auth.RequireSession(app.RevokeJournalShare))

	// THE ONLY ROUTES IN THE APP BEHIND NO MIDDLEWARE AT ALL. Not
	// RequireSession, not OptionalSession, not even to slide an expiry: a
	// shared link has to behave identically for a stranger and for the owner
	// reading their own, and a wrapper that looked for a session would be a
	// difference between the two waiting to become a bug. The token in the
	// path is the whole authorisation, and internal/share is what checks it.
	//
	// The POST is the password gate and is the one mutation in this block --
	// which is why /share/ is not and could not be a /fragment/ prefix. It is a
	// plain form post answered with a 303, because the share layout loads no
	// JavaScript and a gate that needs a script does not open without one.
	//
	// The two image routes exist because the page's pictures have to come from
	// somewhere a signed-out reader can reach, and they are separate routes
	// rather than a relaxation of /characters/{id}/journal/... so that route
	// stays exactly as private as it is. Both are gated by the same cookie the
	// page is: a password on the page with open images would be a locked door
	// beside an open window.
	//
	// The avatar route takes no id. A share names one character and a character
	// has one portrait, so there is nothing in the path to tamper with.
	mux.HandleFunc("GET /share/{token}", app.SharePage)
	mux.HandleFunc("POST /share/{token}", app.UnlockShare)
	mux.HandleFunc("GET /share/{token}/avatar", app.GetShareAvatar)
	mux.HandleFunc("GET /share/{token}/images/{assetId}", app.GetShareImage)

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
	// The journal list, filtered by ?q=. It is the one fragment here that reads
	// parameters, and both are checked before anything is queried: the character
	// must parse as a ULID and the term must be no longer than the box that
	// sends it. The search stays a GET returning the same component the page
	// renders, which is exactly what this prefix is for.
	mux.HandleFunc("GET /fragment/character/journal-entries", auth.Fragment(app.JournalEntriesFragment))
	// The share dialog, in whichever of its two states the entry is in. It
	// reads both ids from the query string and neither from a path, because
	// this is not the entry's URL -- it is a dialog about the entry.
	mux.HandleFunc("GET /fragment/character/journal-share", auth.Fragment(app.JournalShareFragment))

	// Subtree pattern, so it takes any /fragment/ path the four above did not.
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
