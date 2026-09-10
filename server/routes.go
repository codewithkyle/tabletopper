package main

import (
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/controllers"
	"tabletopper/internal/middleware"
)

// handler is what the server actually serves: the URL space below, wrapped in
// the two things that apply to all of it.
//
// THE ORDER IS THE POINT OF IT BEING ONE FUNCTION. CrossOriginProtection is
// Go's own check of Sec-Fetch-Site, and of Origin against Host, on every
// non-safe request -- the second layer under SameSite=Lax on the session
// cookie, which is otherwise the whole of the app's CSRF defence. A request
// carrying neither header passes, so curl and anything else that is not a
// browser is unaffected; it does not cover GET by design, which is why /logout
// is a POST.
//
// IT IS STRICTER THAN SameSite=Lax AND THAT IS WHAT MAKES IT WORTH ADDING. Lax
// treats every host under one registrable domain as the same site; this refuses
// a mutation whose Sec-Fetch-Site says `same-site` as readily as one that says
// `cross-site`, so only same-ORIGIN gets through. Nothing today notices,
// because the app is one host -- but serving it from a second name, or from
// behind a proxy that rewrites the Host, would refuse every mutation until that
// origin is named here with csrf.AddTrustedOrigin. Behind Cloudflare with a
// matching Host there is nothing to add.
//
// The one non-safe request a stranger makes is POST /share/{token}, submitted
// from the gate page itself: same origin, so it passes. The R2 presigned PUT
// goes browser-to-bucket and never reaches this handler at all.
//
// SecurityHeaders is outermost so its floor lands on every response, the 403
// the check above writes included -- a refusal is still a document a browser
// renders.
//
// It is a function rather than three lines in main so that a test can drive
// the real chain. A test that rebuilt the wrapping itself would pass while
// main served the bare mux.
func handler(app *controllers.App, auth middleware.Auth) http.Handler {
	csrf := http.NewCrossOriginProtection()

	return middleware.SecurityHeaders(csrf.Handler(routes(app, auth)))
}

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
	// POST, because it is a state change. A GET here logs somebody out from any
	// cross-site link or redirect they follow -- SameSite=Lax sends the session
	// cookie on a top-level GET navigation, and the cross-origin check in main
	// is defined never to cover safe methods. The badge on the homepage posts a
	// one-button form instead of linking.
	mux.HandleFunc("POST /logout", app.Logout)

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
	// The account's own picture, which overrides the one Clerk supplies. It is a
	// mutation answering with the avatar it just changed, so it keeps a resource
	// URL and stays off /fragment/.
	mux.HandleFunc("POST /account/avatar", auth.RequireSession(app.UploadAccountAvatar))

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
	// Sharing the sheet: the owner's two mutations, and nothing else -- the
	// reader's routes are the /share/ block below, which both kinds of share go
	// through. Like the journal's pair they keep the resource's own URL and stay
	// off /fragment/, and like them they answer with the share dialog in
	// whatever state the character is now in, which is the exception the
	// fragment rules name. The GET that opens that dialog is a fragment and is
	// registered with the rest of them further down.
	//
	// "share" is a literal segment beside the panel names below and cannot
	// collide with one: no panel is called share, and the mux matches literals
	// before it matches anything else.
	mux.HandleFunc("POST /characters/{id}/share", auth.RequireSession(app.CreateCharacterShare))
	mux.HandleFunc("DELETE /characters/{id}/share", auth.RequireSession(app.RevokeCharacterShare))

	// The Markdown download, which is a representation of the character rather
	// than a piece of a page -- so it keeps the resource's own URL and stays
	// off /fragment/, like the image routes. The extension is in the path
	// because it is what a browser, an editor and a vault all read to decide
	// what the file is, and because "export" alone at that depth would look
	// like another panel.
	//
	// It is RequireSession and not RequireSessionOr404: the link is an anchor
	// somebody clicks, so a session that has expired should land them on the
	// sign-in page rather than on nothing at all.
	mux.HandleFunc("GET /characters/{id}/export.md", auth.RequireSession(app.ExportCharacter))

	mux.HandleFunc("POST /characters/{id}/identity", auth.RequireSession(app.SaveCharacterIdentity))
	mux.HandleFunc("POST /characters/{id}/abilities", auth.RequireSession(app.SaveCharacterAbilities))
	mux.HandleFunc("POST /characters/{id}/core-stats", auth.RequireSession(app.SaveCharacterCoreStats))
	mux.HandleFunc("POST /characters/{id}/vitals", auth.RequireSession(app.SaveCharacterVitals))
	mux.HandleFunc("POST /characters/{id}/proficiencies", auth.RequireSession(app.SaveCharacterProficiencies))
	mux.HandleFunc("POST /characters/{id}/personality", auth.RequireSession(app.SaveCharacterPersonality))
	mux.HandleFunc("POST /characters/{id}/appearance", auth.RequireSession(app.SaveCharacterAppearance))
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

	// Attacks are the same shape as inventory -- a collection and a member,
	// because the row is the unit of work -- and differ in where they are read:
	// these rows are edited on the Character tab itself rather than on a tab of
	// their own, so there is no page route above them.
	mux.HandleFunc("POST /characters/{id}/attacks", auth.RequireSession(app.AddAttack))
	mux.HandleFunc("POST /characters/{id}/attacks/{attackId}", auth.RequireSession(app.SaveAttack))
	mux.HandleFunc("DELETE /characters/{id}/attacks/{attackId}", auth.RequireSession(app.DeleteAttack))

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

	// THE READER'S FIVE ROUTES, WHICH SERVE ALL THREE KINDS OF SHARE. A token
	// names a row and the row says whether it opens as a journal entry, a
	// character sheet or a monster, so there is one URL space here and not
	// three -- and nothing in the path that could be edited into asking for
	// another.
	//
	// THE TOKEN IS THE AUTHORISATION ON ALL OF THEM, and internal/share is what
	// checks it. Four of the five ask for no session at all: a shared link has
	// to behave the same for a stranger and for the owner reading their own,
	// and a wrapper that decided anything from a session would be a difference
	// between the two waiting to become a bug.
	//
	// THE PAGE IS OptionalSession AND THE IMPORT IS THE REASON. A shared
	// monster offers a signed-in reader a copy of it for their own manual, and
	// the share layout ships no JavaScript, so there is no second request that
	// could go and ask who is reading. The session decides exactly one thing --
	// whether that button is drawn -- and the entry, the sheet and the monster
	// are identical for everybody either way.
	//
	// The POST is the password gate, a plain form post answered with a 303,
	// because the share layout loads no JavaScript and a gate that needs a
	// script does not open without one. The import is a plain form post for the
	// same reason, and it is the one route in this block that requires a
	// session, because it is the one that writes -- into the reader's own
	// manual, never into the owner's.
	//
	// Neither of them could live under /fragment/: the prefix is for GETs that
	// return partial HTML, and these are mutations that answer with a redirect.
	//
	// The two image routes exist because the page's pictures have to come from
	// somewhere a signed-out reader can reach, and they are separate routes
	// rather than a relaxation of /characters/{id}/journal/... so that route
	// stays exactly as private as it is. Both are gated by the same cookie the
	// page is: a password on the page with open images would be a locked door
	// beside an open window.
	//
	// The images route serves a journal share only, and refuses the other two
	// rather than letting them miss; their one picture each is the portrait.
	//
	// The portrait route takes no id and serves all three kinds. A share names
	// one thing and that thing has one picture, so there is nothing in the path
	// to tamper with.
	//
	// The export answers with a Markdown file rather than a page, and it is
	// behind the same password the page is: a link somebody was handed but
	// never unlocked would otherwise give up the whole sheet as a download. It
	// serves a shared monster and a shared sheet; a shared journal entry is
	// refused, for the reason ExportShare gives.
	mux.HandleFunc("GET /share/{token}", auth.OptionalSession(app.SharePage))
	mux.HandleFunc("POST /share/{token}", app.UnlockShare)
	mux.HandleFunc("POST /share/{token}/import", auth.RequireSession(app.ImportSharedMonster))
	mux.HandleFunc("GET /share/{token}/portrait", app.GetSharePortrait)
	mux.HandleFunc("GET /share/{token}/export.md", app.ExportShare)
	mux.HandleFunc("GET /share/{token}/images/{assetId}", app.GetShareImage)

	// The account settings saves. Four columns on the users row, no path
	// parameter, and no id anywhere: the only account a session can change is
	// its own, so naming one in the URL would create a way to ask for another.
	//
	// The welcome pair are separate routes rather than one route reading which
	// button was pressed, because they do different things to different columns
	// and only one of them reads the form. A single handler branching on a
	// submit value would have to be trusted not to write the pickers on the
	// path that means "I did not answer these".
	mux.HandleFunc("POST /account/settings", auth.RequireSession(app.SaveAccountSettings))
	mux.HandleFunc("POST /account/welcome", auth.RequireSession(app.CompleteOnboarding))
	mux.HandleFunc("POST /account/welcome/skip", auth.RequireSession(app.DismissOnboarding))

	// THE MANUAL, WHICH IS THE ROSTER'S SHAPE FOR MONSTERS: a page of cards, a
	// dialog that creates one from a name, an editor, a delete, and the picture
	// upload that sits on the card the way the roster's avatar upload does.
	//
	// Creation has no page, for the reason the character's does not: it is a
	// dialog carrying one field, served by the fragment route below, and this
	// takes the name it collects and redirects to the editor.
	//
	// The delete answers 200 and not 204 -- noSwap lists 204, and a status in
	// that list overrides the hx-swap="delete" on the button, which would leave
	// the card on screen after the monster was gone.
	mux.HandleFunc("GET /monsters", auth.RequireSession(app.MonstersPage))
	mux.HandleFunc("POST /monsters", auth.RequireSession(app.NewMonsterForm))
	mux.HandleFunc("GET /monsters/{id}/edit", auth.RequireSession(app.MonsterPage))
	mux.HandleFunc("DELETE /monsters/{id}", auth.RequireSession(app.DeleteMonster))
	mux.HandleFunc("POST /monsters/{id}/image", auth.RequireSession(app.UploadMonsterImage))

	// Sharing a monster: the owner's two mutations, and nothing else -- the
	// reader's routes are the /share/ block above, which all three kinds of
	// share go through. Like the character's pair they keep the resource's own
	// URL and stay off /fragment/, and like them they answer with the share
	// dialog in whatever state the monster is now in, which is the exception
	// the fragment rules name. The GET that opens that dialog is a fragment and
	// is registered with the rest of them further down.
	//
	// "share" is a literal segment beside the panel names below and collides
	// with none of them: no panel is called share, and the mux matches literals
	// before it matches anything else.
	mux.HandleFunc("POST /monsters/{id}/share", auth.RequireSession(app.CreateMonsterShare))
	mux.HandleFunc("DELETE /monsters/{id}/share", auth.RequireSession(app.RevokeMonsterShare))

	// The stat block as Markdown, which is the character's export route with a
	// monster under it -- see that one for why the extension is in the path.
	mux.HandleFunc("GET /monsters/{id}/export.md", auth.RequireSession(app.ExportMonster))

	// The editor is ONE page and not a page per tab, because a stat block is one
	// screen: everything a monster has fits beside the block it renders, so
	// there is nothing to navigate between.
	//
	// The panels are the character sheet's shape -- each owns a disjoint set of
	// columns and writes only those -- and mutations, so they keep resource URLs
	// and stay off /fragment/. The reply is a toast, the panel's error block
	// rendered empty, and the out-of-band swaps that redraw the block beside it.
	//
	// Only the bonuses route takes its panel name from the path, and it is the
	// character sheet's own allowlist doing the checking, because these two
	// grids ARE that sheet's two grids.
	//
	// Every literal in the third segment is distinct from every other and no
	// wildcard sits there, so the mux has nothing to disambiguate.
	mux.HandleFunc("POST /monsters/{id}/identity", auth.RequireSession(app.SaveMonsterIdentity))
	mux.HandleFunc("POST /monsters/{id}/abilities", auth.RequireSession(app.SaveMonsterAbilities))
	mux.HandleFunc("POST /monsters/{id}/combat", auth.RequireSession(app.SaveMonsterCombat))
	mux.HandleFunc("POST /monsters/{id}/defenses", auth.RequireSession(app.SaveMonsterDefenses))
	mux.HandleFunc("POST /monsters/{id}/description", auth.RequireSession(app.SaveMonsterDescription))
	mux.HandleFunc("POST /monsters/{id}/bonuses/{kind}", auth.RequireSession(app.SaveMonsterBonuses))

	// The seven sections of the stat block, where the row is the unit of work
	// rather than the panel -- the collection-and-member pair attacks and
	// inventory already use, with the section in the path.
	//
	// The kind is there for the reason the spell level is: a row cannot change
	// section, so it identifies the row as much as its id does, and it is what
	// tells an add which of the seven containers on the page to append to. It is
	// matched against a Go allowlist before any statement runs.
	//
	// "actions" is a literal at the same depth as the panel names above and
	// collides with none of them: no panel is called actions.
	mux.HandleFunc("POST /monsters/{id}/actions/{kind}", auth.RequireSession(app.AddMonsterAction))
	mux.HandleFunc("POST /monsters/{id}/actions/{kind}/{actionId}", auth.RequireSession(app.SaveMonsterAction))
	mux.HandleFunc("DELETE /monsters/{id}/actions/{kind}/{actionId}", auth.RequireSession(app.DeleteMonsterAction))

	// THE ROOM SHELL, WHICH IS THE VIRTUAL TABLETOP WITH NOTHING LIVE IN IT YET.
	// A room is a thing a GM keeps rather than a session they start, so /rooms
	// is a list of them in the roster's shape: cards, and a one-field dialog
	// above them that creates one. The table itself, the socket and the canvas
	// arrive in later phases and arrive inside GET /rooms/{id}.
	//
	// CREATION HAS NO PAGE, for the reason the character's and the monster's do
	// not: it is a dialog carrying one field, served by the fragment route
	// further down, and this takes the name it collects and redirects to the
	// room.
	//
	// "join" IS A LITERAL WHERE {id} GOES, and the mux prefers the literal --
	// the same trust the slot save and the two share pairs depend on. If it
	// ever stopped, every join would arrive at RoomPage with "join" as the id
	// and be redirected to itself. The routes test pins it.
	//
	// GET /rooms/join/{code} PREFILLS AND NEVER JOINS. A GET that seated
	// somebody at a table would be a state change behind a link, which is the
	// rule that put /logout on POST -- and a room code travels in exactly the
	// kind of chat message a link preview crawler follows.
	//
	// The lock pair answer with the control they just changed, which is the
	// mutation case the fragment rules name. The close, the reopen, the leave
	// and the delete answer with a redirect or with nothing, because each of
	// them takes the page away.
	//
	// The delete answers 200 and not 204 -- noSwap lists 204, and a status in
	// that list overrides the hx-swap="delete" on the button, which would leave
	// the card on screen after the room was gone.
	mux.HandleFunc("GET /rooms", auth.RequireSession(app.RoomsPage))
	mux.HandleFunc("POST /rooms", auth.RequireSession(app.NewRoomForm))
	mux.HandleFunc("GET /rooms/join", auth.RequireSession(app.JoinRoomPage))
	mux.HandleFunc("GET /rooms/join/{code}", auth.RequireSession(app.JoinRoomPage))
	mux.HandleFunc("POST /rooms/join", auth.RequireSession(app.JoinRoomForm))
	mux.HandleFunc("GET /rooms/{id}", auth.RequireSession(app.RoomPage))
	mux.HandleFunc("POST /rooms/{id}/lock", auth.RequireSession(app.LockRoom))
	mux.HandleFunc("POST /rooms/{id}/unlock", auth.RequireSession(app.UnlockRoom))
	mux.HandleFunc("POST /rooms/{id}/close", auth.RequireSession(app.CloseRoom))
	mux.HandleFunc("POST /rooms/{id}/open", auth.RequireSession(app.OpenRoom))
	mux.HandleFunc("POST /rooms/{id}/leave", auth.RequireSession(app.LeaveRoom))
	mux.HandleFunc("DELETE /rooms/{id}", auth.RequireSession(app.DeleteRoom))

	// THE KICK IS A POST AND NOT A SOCKET COMMAND, even though player.kick is
	// one of the commands a browser may send. The button lives in the Player
	// List window, which is an ordinary htmx fragment, and routing it through
	// HTTP is what buys hx-confirm -- the app's one gate in front of a
	// destructive action, and a gate this needs. Sending it over the socket
	// would mean a click handler in the room bundle plus a second way to open
	// the confirm dialog, for a mutation that happens about twice a year.
	//
	// It answers with the member list it just changed, which is the mutation
	// case the fragment rules name. The socket says the same thing a moment
	// later -- player.left raises room:players and the window refetches -- but
	// a GM whose own connection has dropped still sees the person go.
	mux.HandleFunc("POST /rooms/{id}/players/{player}/kick", auth.RequireSession(app.KickPlayer))

	// THE TABLE'S CONFIGURATION, and every one of these is HTTP for the reason
	// the kick above is: the controls are htmx, and a control that posted over
	// the socket would need its own confirm, its own way to report a refusal
	// and its own way to draw a form with errors in it.
	//
	// A LAYER IS A RESOURCE AND ITS MAP IS A RESOURCE OF ITS OWN. Setting one
	// is a POST to .../map and clearing it is a DELETE of the same URL, rather
	// than two verbs on the layer -- because clearing a layer's map and
	// deleting the layer are different destructions and a GM who confuses them
	// loses an encounter.
	//
	// NONE OF THEM ANSWERS WITH MARKUP except the grid, which answers with its
	// error block. Every command here ends in table.updated, the windows
	// refetch on it, and a reply carrying the new list would leave a second
	// tab showing the old one.
	mux.HandleFunc("POST /rooms/{id}/layers", auth.RequireSession(app.AddLayer))
	mux.HandleFunc("DELETE /rooms/{id}/layers/{layer}", auth.RequireSession(app.RemoveLayer))
	mux.HandleFunc("PATCH /rooms/{id}/layers/{layer}/name", auth.RequireSession(app.RenameLayer))
	mux.HandleFunc("POST /rooms/{id}/layers/{layer}/move", auth.RequireSession(app.MoveLayer))
	mux.HandleFunc("POST /rooms/{id}/layers/{layer}/activate", auth.RequireSession(app.ActivateLayer))
	mux.HandleFunc("POST /rooms/{id}/layers/{layer}/map", auth.RequireSession(app.SetLayerMap))
	mux.HandleFunc("DELETE /rooms/{id}/layers/{layer}/map", auth.RequireSession(app.ClearLayerMap))
	mux.HandleFunc("POST /rooms/{id}/layers/{layer}/maps", auth.RequireSession(app.UploadRoomMap))
	mux.HandleFunc("POST /rooms/{id}/layers/{layer}/maps/{asset}", auth.RequireSession(app.RetryRoomMapTiling))
	mux.HandleFunc("POST /rooms/{id}/grid", auth.RequireSession(app.SetRoomGrid))
	mux.HandleFunc("POST /rooms/{id}/fog/fill", auth.RequireSession(app.FillLayerFog))
	mux.HandleFunc("POST /rooms/{id}/fog/clear", auth.RequireSession(app.ClearLayerFog))
	mux.HandleFunc("POST /rooms/{id}/tabletop/clear", auth.RequireSession(app.ClearTabletop))

	// WHAT IS ON THE TABLE. Same rule as the layer routes above and for the
	// same reason: these are the DOM's controls, so they are HTTP. What the
	// socket carries is what originates on the CANVAS -- a drag, a placement
	// click -- and neither of those has a form or a confirm dialog in front of
	// it.
	//
	// THE TWO LIST ROUTES TAKE ids AS A REPEATED FORM VALUE rather than one id
	// in the path, because the canvas overlay sends a whole selection to both.
	// The pawn dialog sends a list of one, which is why there is no second pair
	// of routes for the single case.
	//
	// HIT POINTS AND THE NAME HAVE ROUTES OF THEIR OWN, and neither is a
	// duplicate of the pawn's own POST above it. That one is the panel's editor
	// and it posts the WHOLE form -- including the conditions, which it replaces
	// wholesale -- so a control sending one field to it would take every chip
	// off the goblin on its way past. The hit-point boxes also take arithmetic
	// and fire on blur rather than on a debounced keystroke, and the name is
	// changed in a dialog rather than in the panel at all.
	//
	// THE THREE OF THEM ANSWER WITH THE PANEL'S ERROR SLOT AND NOT WITH THE
	// PANEL. Nothing in that window is saved by pressing anything, so a reply
	// that swapped it would regularly replace the field somebody had moved on
	// to; the socket is what brings every open copy back into step, including
	// the one the change came from.
	// THE SPAWN DIALOG ADDING TO ITSELF. Each of these writes something the
	// ACCOUNT owns -- a token, a face, a monster -- and answers with the card
	// or the dialog this room's GM is looking at, which is why the room is in
	// the path. It is UploadRoomMap's trade: identical work to the asset
	// manager's own route, a different representation on the way back.
	mux.HandleFunc("POST /rooms/{id}/spawn/tokens", auth.RequireSession(app.UploadSpawnToken))
	mux.HandleFunc("POST /rooms/{id}/spawn/avatars", auth.RequireSession(app.UploadSpawnAvatar))
	mux.HandleFunc("POST /rooms/{id}/spawn/monsters", auth.RequireSession(app.CreateSpawnMonster))
	mux.HandleFunc("POST /rooms/{id}/pawns/party", auth.RequireSession(app.SpawnParty))
	mux.HandleFunc("POST /rooms/{id}/pawns/layer", auth.RequireSession(app.MovePawnsToLayer))
	mux.HandleFunc("POST /rooms/{id}/pawns/shown", auth.RequireSession(app.SetPawnsShown))
	mux.HandleFunc("DELETE /rooms/{id}/pawns", auth.RequireSession(app.RemovePawns))
	mux.HandleFunc("POST /rooms/{id}/pawns/{pawn}", auth.RequireSession(app.UpdatePawn))
	mux.HandleFunc("POST /rooms/{id}/pawns/{pawn}/hp", auth.RequireSession(app.UpdatePawnHP))
	mux.HandleFunc("POST /rooms/{id}/pawns/{pawn}/name", auth.RequireSession(app.RenamePawn))

	// THE TURN ORDER. Eight routes, and seven of them are the GM's; the core
	// refuses the rest of the room rather than the mux, which is what lets Next
	// be the one exception without a rule of its own here -- whoever owns a
	// pawn in the acting line may end its turn.
	//
	// SYNC AND CLEAR ARE POSTS BECAUSE THEY ARE MENU ITEMS. A menu item is a
	// button carrying hx-post; making a clear the one DELETE would mean a
	// second branch in the item markup for one route. It is ClearTabletop's
	// reasoning, one menu along.
	//
	// EVERY ONE OF THEM ANSWERS 204 AND REDRAWS NOTHING. Each ends in
	// initiative.updated, the strip listens for it, and the tab that sent the
	// command is corrected by the same event as the tab beside it.
	mux.HandleFunc("POST /rooms/{id}/initiative", auth.RequireSession(app.AddInitiative))
	mux.HandleFunc("POST /rooms/{id}/initiative/sync", auth.RequireSession(app.SyncInitiative))
	mux.HandleFunc("POST /rooms/{id}/initiative/order", auth.RequireSession(app.OrderInitiative))
	mux.HandleFunc("POST /rooms/{id}/initiative/next", auth.RequireSession(app.NextInitiative))
	mux.HandleFunc("POST /rooms/{id}/initiative/clear", auth.RequireSession(app.ClearInitiative))
	mux.HandleFunc("POST /rooms/{id}/initiative/{entry}/activate", auth.RequireSession(app.ActivateInitiative))
	mux.HandleFunc("DELETE /rooms/{id}/initiative/{entry}", auth.RequireSession(app.RemoveInitiative))

	// The room's live connection, and the only route in the app that answers
	// with neither a document nor a fragment of one.
	//
	// IT IS NOT "/rooms/{id}/socket", WHICH IS WHERE IT BELONGS AND CANNOT GO.
	// That pattern and "GET /rooms/join/{code}" above both match
	// "/rooms/join/socket" -- each has a literal where the other has a
	// wildcard, so neither is more specific and ServeMux panics at
	// registration. net/http has no way to settle that: a third, more specific
	// pattern does not resolve a conflict the way it does in some routers. So
	// the socket takes a prefix of its own, which every future GET under a room
	// id would otherwise have to fight the same battle for.
	//
	// THE PREFIX MEANS WHAT /fragment/ MEANS, one level up: it names a kind of
	// response rather than a kind of resource. /fragment/ is a GET that returns
	// partial HTML; this is a GET that returns no HTML at all, and putting it
	// under /fragment/ would break that prefix's one promise.
	//
	// RequireSessionOr404 RATHER THAN RequireSession, because a redirect to
	// /sign-in is not something a WebSocket upgrade can follow -- the browser
	// reports a failed handshake and the client retries it on its backoff
	// forever, against a sign-in page. A 404 is a refusal the client can read.
	mux.HandleFunc("GET /socket/room/{id}", auth.RequireSessionOr404(app.RoomSocket))

	// THE ASSET MANAGER IS A PAGE PER KIND, joined by the sub-nav across the
	// top. /assets is a redirect onto the first of them rather than an index:
	// there is nothing to show above the kinds that the tab strip does not
	// already show, and a page whose whole content is four links to the pages
	// beneath it is a page nobody wants to land on twice.
	//
	// FOUR LITERAL ROUTES RATHER THAN "GET /assets/{kind}". The kinds are a
	// closed set whose handlers do not resemble each other -- a map is tiled by
	// a background worker, music is not an image at all -- so a wildcard would
	// be matched against an allowlist and then switched on, which is a longer
	// way of writing what the mux does here for nothing.
	mux.HandleFunc("GET /assets", auth.RequireSession(app.AssetsPage))
	mux.HandleFunc("GET /assets/maps", auth.RequireSession(app.MapAssetsPage))
	mux.HandleFunc("GET /assets/tokens", auth.RequireSession(app.TokenAssetsPage))
	mux.HandleFunc("GET /assets/avatars", auth.RequireSession(app.AvatarAssetsPage))
	mux.HandleFunc("GET /assets/music", auth.RequireSession(app.MusicAssetsPage))

	// THE LIBRARY KINDS, which are the two that are one stored image and
	// nothing else. Each is the collection-and-member pair every other resource
	// here uses, and the two sets are identical but for the segment -- one set
	// of handlers serves both, with the kind bound at registration rather than
	// read from the path. See internal/controllers/library-assets.go.
	//
	// THE KIND IN THE PATH IS ENFORCED AND NOT DECORATIVE. Every statement
	// behind these carries the type, so a token's id sent to an avatars route
	// is a 404 rather than a token quietly renamed, replaced or deleted through
	// the wrong page.
	//
	// The upload and the replace both answer with the card they made or
	// changed, which is the mutation case the fragment rules name. The delete
	// answers 200 and not 204 -- noSwap lists 204, and a status in that list
	// overrides the hx-swap="delete" on the button, which would leave the card
	// on screen after the asset was gone.
	//
	// Music has none of these yet: it is not an image, so it shares no handler
	// with either of them, and its bytes will not pass through this process at
	// all.
	mux.HandleFunc("POST /assets/tokens", auth.RequireSession(app.UploadToken))
	mux.HandleFunc("POST /assets/tokens/{id}", auth.RequireSession(app.ReplaceToken))
	mux.HandleFunc("PATCH /assets/tokens/{id}/name", auth.RequireSession(app.RenameToken))
	mux.HandleFunc("DELETE /assets/tokens/{id}", auth.RequireSession(app.DeleteToken))

	mux.HandleFunc("POST /assets/avatars", auth.RequireSession(app.UploadAvatar))
	mux.HandleFunc("POST /assets/avatars/{id}", auth.RequireSession(app.ReplaceAvatar))
	mux.HandleFunc("PATCH /assets/avatars/{id}/name", auth.RequireSession(app.RenameAvatar))
	mux.HandleFunc("DELETE /assets/avatars/{id}", auth.RequireSession(app.DeleteAvatar))

	// MUSIC, WHOSE UPLOAD IS TWO REQUESTS BECAUSE ITS BYTES NEVER COME HERE. A
	// track is 115 to 175 MB, so the browser PUTs it straight to R2 through a
	// presigned URL: the first route writes the row that claims the key and
	// hands back the signature, and the second looks in the bucket afterwards
	// and finishes the row. See internal/controllers/music-assets.go.
	//
	// THE FIRST ANSWERS JSON AND THE SECOND ANSWERS THE CARD. Neither could
	// live under /fragment/: the prefix is for GETs that return partial HTML,
	// and these are mutations -- one of which returns no HTML at all.
	//
	// "confirm" is a literal in the third segment, where no wildcard sits, so
	// it cannot be taken for an id.
	mux.HandleFunc("POST /assets/music", auth.RequireSession(app.StartMusicUpload))
	mux.HandleFunc("POST /assets/music/{id}/confirm", auth.RequireSession(app.ConfirmMusicUpload))
	mux.HandleFunc("PATCH /assets/music/{id}/name", auth.RequireSession(app.RenameMusic))
	mux.HandleFunc("DELETE /assets/music/{id}", auth.RequireSession(app.DeleteMusic))

	// The player's source. It is a 302 onto a freshly signed URL rather than a
	// proxy of the bytes: an <audio> element seeks by asking for byte ranges,
	// R2 answers those natively, and a browser repeats a GET's headers through
	// a redirect -- so the Range survives the hop and no audio ever passes
	// through this process.
	//
	// It is also what keeps a signature from going stale in the markup. Every
	// request the player makes comes back through here and gets a URL minted a
	// moment earlier.
	//
	// RequireSessionOr404 like the image routes, and for the same reason: this
	// is the src of a media element, so a redirect to the sign-in page renders
	// as a player that will not play rather than as a sign-in page.
	mux.HandleFunc("GET /assets/music/{id}/audio", auth.RequireSessionOr404(app.GetMusicAudio))
	mux.HandleFunc("POST /assets/maps", auth.RequireSession(app.UploadMap))
	mux.HandleFunc("DELETE /assets/maps/{id}", auth.RequireSession(app.DeleteMap))
	mux.HandleFunc("POST /assets/maps/{id}", auth.RequireSession(app.ReplaceMap))
	mux.HandleFunc("PATCH /assets/maps/{id}/name", auth.RequireSession(app.ReplaceMapName))
	mux.HandleFunc("POST /assets/maps/{id}/tiles", auth.RequireSession(app.RetryMapTiling))

	// One tile of one generation of one map's pyramid. It answers image/webp,
	// so it is here beside the map it belongs to rather than under /fragment/,
	// and it is RequireSessionOr404 like the two routes below it -- a redirect
	// to the sign-in page renders as a broken image.
	//
	// THE LAST SEGMENT IS ONE WILDCARD AND NOT TWO. The URL it serves is
	// .../{z}/{x}_{y}.webp, but a ServeMux wildcard has to be a whole path
	// segment: writing that pattern out panics here at registration and the
	// server does not start. The handler splits the segment.
	mux.HandleFunc("GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}", auth.RequireSessionOr404(app.GetMapTile))

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
	// The sheet's share dialog, which is the one above with one id instead of
	// two. It reads the character from the query string rather than a path for
	// the same reason: this is not the character's URL, it is a dialog about the
	// character, and the Share button that opens it is on all five editor tabs.
	mux.HandleFunc("GET /fragment/character/share", auth.Fragment(app.CharacterShareFragment))
	// The account settings dialog. It reads no query parameters at all -- the
	// four values it shows come off the session, which carries them on every
	// request -- so there is nothing here to validate and nothing a caller
	// could ask for that is not their own.
	mux.HandleFunc("GET /fragment/account/settings", auth.Fragment(app.AccountSettingsFragment))
	// The welcome dialog, which the homepage opens by itself for an account
	// that has never answered it. Nothing links here and nothing needs to: the
	// signal is a column, so the page decides rather than the URL.
	mux.HandleFunc("GET /fragment/account/welcome", auth.Fragment(app.AccountWelcomeFragment))
	// The new-monster dialog, which is the character's with its own panel name.
	mux.HandleFunc("GET /fragment/monster/new", auth.Fragment(app.NewMonsterFragment))
	// The monster's share dialog, which is the sheet's with a monster id. It
	// reads that id from the query string rather than a path for the same
	// reason: this is not the monster's URL, it is a dialog about the monster.
	mux.HandleFunc("GET /fragment/monster/share", auth.Fragment(app.MonsterShareFragment))
	// The manual's grid, filtered by ?q=. It is the journal search's shape with
	// one parameter instead of two: there is no id in this URL, because the
	// manual is the account's rather than any character's, and the owner comes
	// off the session. The term is checked against the name column's width
	// before anything is queried.
	mux.HandleFunc("GET /fragment/monster/list", auth.Fragment(app.MonsterListFragment))
	// One monster's stat block, for the dialog the manual's View button opens.
	// It reads the monster from the query string rather than a path because this
	// is not the monster's URL -- it is a representation of it that something
	// else opens, which is the same reason the two share dialogs above do it.
	// It is also the lookup a pawn will make when the VTT exists, since a pawn
	// holds nothing but this id and its own instance stats.
	mux.HandleFunc("GET /fragment/monster/stat-block", auth.Fragment(app.MonsterStatBlockFragment))
	// One map's card, which is what a card whose tiling job has not finished
	// asks for every couple of seconds until it has. It is the only fragment
	// here that is fetched by a timer rather than by something the owner did,
	// which is why the element carrying the poll also carries data-quiet: see
	// public/js/loading.js for what that suppresses.
	//
	// It takes the map from the path and nothing from the query string,
	// because this IS the card's own URL -- the one representation of it that
	// is not the asset manager page. The two dialogs above read a query string
	// instead precisely because they are not.
	mux.HandleFunc("GET /fragment/assets/maps/{id}/card", auth.Fragment(app.MapCardFragment))

	// The new-room dialog, which is the character's and the monster's with its
	// own panel name and its own action.
	mux.HandleFunc("GET /fragment/room/new", auth.Fragment(app.NewRoomFragment))

	// The player window behind the Room menu, and the first live panel in the
	// app: a socket event fires a DOM event, this element's hx-trigger hears it
	// and refetches. That is the refetch pattern the whole room page is built
	// on -- rendered markup stays on HTTP and the socket carries JSON.
	//
	// IT IS GATED ON MEMBERSHIP AND NOT ON OWNERSHIP, which is what makes it
	// the first fragment here a player may fetch. auth.Fragment answers "who is
	// asking"; which room they are asking about is a query parameter, so the
	// membership check is in the handler beside the parse rather than in a
	// wrapper that would have to parse it a second time.
	mux.HandleFunc("GET /fragment/room/members", auth.Fragment(app.RoomMembersFragment))

	// The active layer's name, in the menu bar, for everybody in the room. It
	// is a fragment rather than a value the client fills in from the store
	// because it is one string that changes when a GM clicks a menu item: the
	// refetch pattern the player list uses costs one GET per floor change per
	// client, and the alternative is a script that has to be kept in step with
	// the reducer to print a name.
	//
	// IT ANSWERS EMPTY FOR A ROOM WITH ONE LAYER, which is nearly every room.
	// A bar that permanently said "Ground floor" would be labelling the only
	// thing there is.
	mux.HandleFunc("GET /fragment/room/layer", auth.Fragment(app.RoomLayerFragment))

	// THE GM'S TWO CONFIGURATION WINDOWS AND THE PICKER ONE OF THEM OPENS.
	// Unlike the members fragment above, these are gated on OWNERSHIP: they are
	// the controls that decide what the table is, and a player who fetched one
	// would be reading the room's configuration. The handlers answer a player
	// with the same 404 they answer a stranger.
	mux.HandleFunc("GET /fragment/room/layers", auth.Fragment(app.RoomLayersFragment))
	mux.HandleFunc("GET /fragment/room/maps", auth.Fragment(app.RoomMapsFragment))
	mux.HandleFunc("GET /fragment/room/map-list", auth.Fragment(app.RoomMapListFragment))
	mux.HandleFunc("GET /fragment/room/map-card", auth.Fragment(app.RoomMapCardFragment))
	mux.HandleFunc("GET /fragment/room/grid", auth.Fragment(app.RoomGridFragment))

	// The pawn fragments, and they split three ways on who may read them.
	//
	// THE SPAWN DIALOG AND ITS RESULTS ARE THE GM'S, like the layer manager
	// above: a player who fetched one would be handed the GM's whole manual.
	//
	// THE PANEL IS ANY MEMBER'S AND IS PROJECTED FOR THEM. It is the one
	// fragment in this file that a player may fetch about a specific thing on
	// the table, and hub.Pawn answers it with the copy their role may see or
	// with nothing at all -- so a hidden monster is a 404 with an empty body,
	// which is also what a pawn id that never existed gets. The two are
	// indistinguishable on purpose: the whole two-audience design says a hidden
	// pawn never reaches a player's browser, and a fragment route is the second
	// door into the state the socket projects on the way out.
	//
	// THE STAT BLOCK IS THE GM'S. The room's monster-health setting exists so a
	// table can hide a monster's hit points; a stat block carries those, its
	// armour class and its legendary actions, so serving one to a player would
	// contradict in one window the setting the GM chose in another.
	mux.HandleFunc("GET /fragment/room/spawn", auth.Fragment(app.RoomSpawnFragment))
	mux.HandleFunc("GET /fragment/room/spawn-list", auth.Fragment(app.RoomSpawnListFragment))
	mux.HandleFunc("GET /fragment/room/spawn-npc", auth.Fragment(app.RoomSpawnNPCFragment))
	mux.HandleFunc("GET /fragment/room/spawn-monster", auth.Fragment(app.RoomSpawnMonsterFragment))
	mux.HandleFunc("GET /fragment/room/pawn", auth.Fragment(app.RoomPawnFragment))
	mux.HandleFunc("GET /fragment/room/pawn/rename", auth.Fragment(app.RoomPawnRenameFragment))
	mux.HandleFunc("GET /fragment/room/condition-row", auth.Fragment(app.RoomConditionRowFragment))
	mux.HandleFunc("GET /fragment/room/stat-block", auth.Fragment(app.RoomStatBlockFragment))

	// The turn order, and the two halves of it split the way the pawn
	// fragments above do.
	//
	// THE STRIP IS EVERYBODY'S AND IS PROJECTED FOR THEM. It is the only live
	// surface on the room page that is neither the canvas nor a window, because
	// a turn order is read by the whole table every few seconds for the minutes
	// a fight lasts. hub.Initiative answers it with the copy the asking role
	// may see: a hidden monster has no line in a player's copy and a hidden
	// goblin is missing from its group's dots, which is the same rule the
	// socket applies on the way out.
	//
	// THE ADD DIALOG IS THE GM'S, like the layer manager: it is a control for
	// the fight rather than a reading of it.
	mux.HandleFunc("GET /fragment/room/initiative", auth.Fragment(app.RoomInitiativeFragment))
	mux.HandleFunc("GET /fragment/room/initiative/entry", auth.Fragment(app.RoomInitiativeEntryFragment))
	mux.HandleFunc("GET /fragment/room/initiative/round", auth.Fragment(app.RoomInitiativeRoundFragment))

	// The grid under one manager page's search box. ONE ROUTE FOR ALL FOUR
	// KINDS, where the pages above are four literal routes -- the pages have
	// four handlers that do not resemble each other, and this is the same work
	// four times with a different statement and a different card in it.
	//
	// ?kind= IS THE ONE PLACE IN THE MANAGER A KIND COMES OFF THE WIRE, so it
	// is matched against the four members before a statement runs; anything
	// else is an empty 404. ?q= is bounded by the name column's width the same
	// way /fragment/monster/list bounds its own.
	mux.HandleFunc("GET /fragment/assets/list", auth.Fragment(app.AssetListFragment))

	// Subtree pattern, so it takes any /fragment/ path the five above did not.
	// Without it these fall to the catch-all on "/" and answer with Go's
	// plain-text 404 page, which is a page-shaped reply to a fragment request.
	mux.HandleFunc("/fragment/", middleware.FragmentNotFound)

	// Static files. The URL prefix is the directory under public/, so one
	// FileServer rooted there covers all four without a StripPrefix each.
	//
	// A PATH ENDING IN A SLASH IS REFUSED BEFORE THE FILE SERVER SEES IT.
	// http.FileServer renders an index for any directory without an
	// index.html, so GET /js/ would list the scripts and GET /css/ the
	// stylesheets. Nothing in here is secret and nothing is served from a
	// directory on purpose, so a listing is only ever a map of the app handed
	// to somebody who asked for one.
	//
	// Cache-Control is an hour, which is the compromise a URL without a
	// fingerprint in it forces. Nothing here is content-addressed -- app.css is
	// app.css across deploys -- so a long max-age would leave a browser holding
	// last week's stylesheet against this week's markup, and no header at all
	// leaves it to a heuristic that differs per browser. An hour is short
	// enough to ride out a deploy and long enough that a session's worth of
	// navigation does not refetch the same four files. Fingerprint the names
	// and this becomes immutable.
	static := http.FileServer(http.Dir("./public"))
	files := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			notFound(w, r)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	}
	for _, prefix := range []string{"/css/", "/js/", "/static/", "/images/"} {
		mux.HandleFunc("GET "+prefix, files)
	}

	return mux
}

func notFound(w http.ResponseWriter, r *http.Request) {
	slog.Warn("404 Not Found", "path", r.URL.Path)
	http.NotFound(w, r)
}
