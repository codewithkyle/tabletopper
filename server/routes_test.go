package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tabletopper/internal/controllers"
	"tabletopper/internal/middleware"
)

// http.ServeMux panics on two patterns that overlap without one being more
// specific, and it does it at registration -- which is boot, in main. This
// builds the whole URL space so that failure lands in `make check` instead of
// on the first deploy. The handlers are never called, so the zero-valued App
// and Auth are enough.
func TestRoutesRegisterWithoutConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()

	routes(&controllers.App{}, middleware.Auth{})
}

// The panel saves and the routes that already lived under /characters/{id} have
// to stay distinguishable. ServeMux accepts all of them, so this checks the one
// thing acceptance does not prove: that a request lands on the pattern it looks
// like it should.
func TestPanelRoutesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)

	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	item := "01BX5ZZKBKACTAV9WEVGEMMVS0"
	asset := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	// A share token is not a ULID: 22 characters of base64url, which is what
	// the reader's routes carry instead of an id.
	token := "yA1rMcJ4TkK9wQ2sVbNpXg"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/characters/" + id + "/avatar", "POST /characters/{id}/avatar"},
		{http.MethodPost, "/characters/" + id + "/identity", "POST /characters/{id}/identity"},
		{http.MethodPost, "/characters/" + id + "/core-stats", "POST /characters/{id}/core-stats"},
		{http.MethodPost, "/characters/" + id + "/bonuses/skills", "POST /characters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/characters/" + id + "/features", "POST /characters/{id}/features"},
		{http.MethodGet, "/characters/" + id + "/edit", "GET /characters/{id}/edit"},
		// The bare path is still a route, but it is a redirect to cantrips
		// rather than a page -- there is no index above the levels, and a
		// bookmark to it should land somewhere.
		{http.MethodGet, "/characters/" + id + "/edit/spells", "GET /characters/{id}/edit/spells"},
		{http.MethodGet, "/characters/" + id + "/edit/spells/0", "GET /characters/{id}/edit/spells/{level}"},
		{http.MethodGet, "/characters/" + id + "/edit/spells/3", "GET /characters/{id}/edit/spells/{level}"},
		{http.MethodGet, "/characters/" + id + "/edit/inventory", "GET /characters/{id}/edit/inventory"},
		// The collection and the member have to stay apart. They differ by one
		// segment, and getting them confused would send an add to the save
		// handler with no itemId to parse.
		{http.MethodPost, "/characters/" + id + "/inventory", "POST /characters/{id}/inventory"},
		{http.MethodPost, "/characters/" + id + "/inventory/" + item, "POST /characters/{id}/inventory/{itemId}"},
		{http.MethodDelete, "/characters/" + id + "/inventory/" + item, "DELETE /characters/{id}/inventory/{itemId}"},
		// Spells are the same collection-and-member pair with the level in
		// between. THE FIRST OF THESE IS THE ONE THAT MATTERS: "slots" and a
		// level occupy the same position, and the mux is being trusted to
		// prefer the literal. If it ever stopped, every slot save would arrive
		// at AddSpell with a level of "slots" and 404 -- which looks like a save
		// that quietly did nothing rather than like a routing bug.
		{http.MethodPost, "/characters/" + id + "/spells/slots/3", "POST /characters/{id}/spells/slots/{level}"},
		{http.MethodPost, "/characters/" + id + "/spells/3", "POST /characters/{id}/spells/{level}"},
		{http.MethodPost, "/characters/" + id + "/spells/3/" + item, "POST /characters/{id}/spells/{level}/{spellId}"},
		{http.MethodDelete, "/characters/" + id + "/spells/3/" + item, "DELETE /characters/{id}/spells/{level}/{spellId}"},
		// The journal repeats the collection-and-member pair, with the page
		// routes one segment deeper under /edit/. The list page and the entry
		// page differ only by that segment, and the mutations differ from both
		// by not carrying /edit/ at all -- so a create arriving at the save
		// handler, or a save arriving at a page, is exactly the confusion this
		// rules out.
		{http.MethodGet, "/characters/" + id + "/edit/journal", "GET /characters/{id}/edit/journal"},
		{http.MethodGet, "/characters/" + id + "/edit/journal/" + item, "GET /characters/{id}/edit/journal/{entryId}"},
		{http.MethodPost, "/characters/" + id + "/journal", "POST /characters/{id}/journal"},
		{http.MethodPost, "/characters/" + id + "/journal/" + item, "POST /characters/{id}/journal/{entryId}"},
		{http.MethodDelete, "/characters/" + id + "/journal/" + item, "DELETE /characters/{id}/journal/{entryId}"},
		// THE TWO SHARES OCCUPY THE SAME POSITION AS EACH OTHER'S SUBJECT. The
		// sheet's is /characters/{id}/share and an entry's is the same word
		// three segments deeper, so a sheet's revoke arriving at the entry's
		// handler would delete a link nobody asked about -- and the two delete
		// different rows on purpose. "share" is also a literal sitting where a
		// panel name goes, which is the same trust in the mux the slot save
		// above depends on.
		{http.MethodPost, "/characters/" + id + "/share", "POST /characters/{id}/share"},
		{http.MethodDelete, "/characters/" + id + "/share", "DELETE /characters/{id}/share"},
		{http.MethodGet, "/characters/" + id + "/export.md", "GET /characters/{id}/export.md"},
		{http.MethodPost, "/characters/" + id + "/journal/" + item + "/share", "POST /characters/{id}/journal/{entryId}/share"},
		{http.MethodDelete, "/characters/" + id + "/journal/" + item + "/share", "DELETE /characters/{id}/journal/{entryId}/share"},
		// An entry's images hang off the member as a sub-collection, so the
		// upload and the entry's own save differ by one segment and the serve
		// route sits two below the member. The mux is being trusted to keep
		// POST .../journal/{entryId} and POST .../journal/{entryId}/images
		// apart -- confusing them would send an upload to SaveJournalEntry,
		// which would read no title and no body off a multipart form and blank
		// the entry the image was going into.
		{http.MethodPost, "/characters/" + id + "/journal/" + item + "/images", "POST /characters/{id}/journal/{entryId}/images"},
		{http.MethodGet, "/characters/" + id + "/journal/" + item + "/images/" + asset, "GET /characters/{id}/journal/{entryId}/images/{assetId}"},
		// The sub-collection has no GET of its own: an entry's images are
		// listed by the markdown that references them, not by a route.
		{http.MethodGet, "/characters/" + id + "/journal/" + item + "/images", "/"},
		// THE MANUAL IS A SECOND TOP-LEVEL COLLECTION, so its three routes have
		// to stay apart from each other the way the roster's do -- and the
		// delete is the one that matters: DELETE /monsters/{id} and POST
		// /monsters differ by a segment, and confusing them would send a delete
		// to the create handler, which reads a form that is not there and
		// answers 422 as though the name were missing.
		{http.MethodGet, "/monsters", "GET /monsters"},
		{http.MethodPost, "/monsters", "POST /monsters"},
		{http.MethodDelete, "/monsters/" + id, "DELETE /monsters/{id}"},
		{http.MethodGet, "/monsters/" + id + "/edit", "GET /monsters/{id}/edit"},
		// The panel saves, which sit at the same depth as /edit and are told
		// apart from it by their literals alone. A save arriving at the editor
		// page would answer a POST with a whole page; the page arriving at a
		// save would write a panel from a form that is not there.
		{http.MethodPost, "/monsters/" + id + "/identity", "POST /monsters/{id}/identity"},
		{http.MethodPost, "/monsters/" + id + "/abilities", "POST /monsters/{id}/abilities"},
		{http.MethodPost, "/monsters/" + id + "/combat", "POST /monsters/{id}/combat"},
		{http.MethodPost, "/monsters/" + id + "/defenses", "POST /monsters/{id}/defenses"},
		{http.MethodPost, "/monsters/" + id + "/description", "POST /monsters/{id}/description"},
		{http.MethodPost, "/monsters/" + id + "/bonuses/skills", "POST /monsters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/bonuses/saving_throws", "POST /monsters/{id}/bonuses/{kind}"},
		// The action rows repeat the collection-and-member pair with the section
		// in between. The collection and the member differ by one segment, and
		// confusing them would send an add to the save handler with no actionId
		// to parse -- which is the same trap the inventory pair sets.
		{http.MethodPost, "/monsters/" + id + "/actions/trait", "POST /monsters/{id}/actions/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/actions/legendary_action", "POST /monsters/{id}/actions/{kind}"},
		{http.MethodPost, "/monsters/" + id + "/actions/trait/" + item, "POST /monsters/{id}/actions/{kind}/{actionId}"},
		{http.MethodDelete, "/monsters/" + id + "/actions/trait/" + item, "DELETE /monsters/{id}/actions/{kind}/{actionId}"},
		// A kind the mux accepts and the allowlist does not. Which of the two
		// refuses it matters: the pattern has to match so the handler gets to
		// answer, rather than the request falling to the catch-all's page-shaped
		// 404.
		{http.MethodPost, "/monsters/" + id + "/actions/mythic_action", "POST /monsters/{id}/actions/{kind}"},
		// A section has no GET of its own: its rows are rendered by the editor,
		// not fetched by a route.
		{http.MethodGet, "/monsters/" + id + "/actions/trait", "/"},
		{http.MethodPost, "/monsters/" + id + "/image", "POST /monsters/{id}/image"},
		// The manual's share pair, which sits where a panel name goes -- the
		// same trust in the mux preferring a literal that the sheet's pair and
		// the slot save depend on. A revoke arriving at a panel save would
		// answer a DELETE by writing columns from a form that is not there.
		{http.MethodPost, "/monsters/" + id + "/share", "POST /monsters/{id}/share"},
		{http.MethodDelete, "/monsters/" + id + "/share", "DELETE /monsters/{id}/share"},
		// The Markdown download, which sits where a panel name goes with a dot
		// in it. The extension is part of the literal, so /monsters/{id}/export
		// is a miss rather than the same route -- which is the point of putting
		// it there: the path says what the file is.
		{http.MethodGet, "/monsters/" + id + "/export.md", "GET /monsters/{id}/export.md"},
		{http.MethodGet, "/monsters/" + id + "/export", "/"},
		{http.MethodPost, "/monsters/" + id + "/export.md", "/"},
		// Creation has no page here either, and "/monsters/new" is the path most
		// likely to be added by accident -- it looks like the matched pair of
		// "/monsters/{id}/edit".
		{http.MethodGet, "/monsters/new", "/"},
		{http.MethodGet, "/fragment/character/new", "GET /fragment/character/new"},
		{http.MethodGet, "/fragment/monster/new", "GET /fragment/monster/new"},
		// The manual's search, whose parameters ride in the query string. A POST
		// to it is not a route at all but the /fragment/ subtree's 404, which is
		// what keeps the prefix meaning "a GET that returns partial HTML".
		{http.MethodGet, "/fragment/monster/share?monster=" + id, "GET /fragment/monster/share"},
		{http.MethodGet, "/fragment/monster/list", "GET /fragment/monster/list"},
		{http.MethodGet, "/fragment/monster/list?q=goblin", "GET /fragment/monster/list"},
		{http.MethodPost, "/fragment/monster/list", "/fragment/"},
		// The stat block dialog, whose parameters ride in the query string like
		// the search's.
		{http.MethodGet, "/fragment/monster/stat-block?monster=" + id, "GET /fragment/monster/stat-block"},
		{http.MethodPost, "/fragment/monster/stat-block", "/fragment/"},
		{http.MethodGet, "/fragment/character/feature-row", "GET /fragment/character/feature-row"},
		{http.MethodGet, "/fragment/character/journal-link", "GET /fragment/character/journal-link"},
		// The journal search. Its parameters ride in the query string, which the
		// mux does not see, so the pattern is the bare path -- and a POST to it
		// is not a route at all but the /fragment/ subtree's 404, which is what
		// keeps the prefix meaning "a GET that returns partial HTML".
		{http.MethodGet, "/fragment/character/journal-entries", "GET /fragment/character/journal-entries"},
		{http.MethodGet, "/fragment/character/journal-entries?character=" + id + "&q=hag", "GET /fragment/character/journal-entries"},
		{http.MethodPost, "/fragment/character/journal-entries", "/fragment/"},
		// None of these is a route any more, so all three fall to the catch-all
		// rather than to one of the above. The first two were the whole-sheet
		// save, which the panels replaced. The third was the create page, which
		// the dialog replaced -- and it is the one most likely to be re-added by
		// accident, because "/characters/new" and "/characters/{id}/edit" look
		// like a matched pair.
		{http.MethodPost, "/characters/" + id + "/rows", "/"},
		{http.MethodPost, "/characters/" + id, "/"},
		{http.MethodGet, "/characters/new", "/"},
		// The whole-sheet spells save, which held all ten levels in one JSON
		// column. Every spell route carries a level now, so the bare collection
		// is a miss.
		{http.MethodPost, "/characters/" + id + "/spells", "/"},
		// Inventory rows are edited through the collection above, not through a
		// GET of their own -- there is no representation of a single item to
		// fetch, so this is a miss rather than a route waiting to be written.
		{http.MethodGet, "/characters/" + id + "/inventory", "/"},
		{http.MethodDelete, "/characters/" + id + "/inventory", "/"},
		// The repeaters' shared route. Features was the last one through it and
		// now has a route naming itself, so the old path is a miss -- and it is
		// worth pinning, because a stale hx-post attribute pointing here would
		// post, 404, and look like a save that quietly did nothing.
		{http.MethodPost, "/characters/" + id + "/rows/features", "/"},
		// Same for the add-row fragment, which no longer takes a ?field=. This
		// one lands on the /fragment/ subtree rather than the root catch-all,
		// which is the difference between a 404 shaped like a page and one
		// shaped like nothing.
		{http.MethodGet, "/fragment/character/info-row", "/fragment/"},
		// And the blank-spell-card fragment. Adding a spell is a POST that
		// answers with the row it created, so there is nothing left to GET.
		{http.MethodGet, "/fragment/character/spell-card", "/fragment/"},
		// THE READER'S BLOCK, WHERE TWO POSTS SIT ONE SEGMENT APART. The bare
		// path is the password gate and the deeper one takes a copy of a shared
		// monster, so confusing them would either check a password against a
		// form that carries none, or copy a monster on somebody typing one in.
		// The portrait is a GET at the same depth as the import, which is the
		// other half of the same question.
		{http.MethodGet, "/share/" + token, "GET /share/{token}"},
		{http.MethodPost, "/share/" + token, "POST /share/{token}"},
		{http.MethodPost, "/share/" + token + "/import", "POST /share/{token}/import"},
		{http.MethodGet, "/share/" + token + "/portrait", "GET /share/{token}/portrait"},
		{http.MethodGet, "/share/" + token + "/export.md", "GET /share/{token}/export.md"},
		{http.MethodGet, "/share/" + token + "/images/" + asset, "GET /share/{token}/images/{assetId}"},
		// The import is a mutation and has no representation to fetch, and the
		// portrait is a representation and is not written by anybody. Both are
		// misses rather than routes waiting to be written.
		{http.MethodGet, "/share/" + token + "/import", "/"},
		{http.MethodPost, "/share/" + token + "/portrait", "/"},
		// The old name of the portrait route, which served a character's avatar
		// before a monster had a picture to serve here too. Nothing links to it
		// -- every shared page builds the URL from the token as it renders --
		// so this is a miss rather than a redirect.
		{http.MethodGet, "/share/" + token + "/avatar", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

// The map's routes, which now go three segments deeper than any other asset
// route. Two things here are worth pinning rather than trusting.
//
// THE RETRY AND A TILE ARE THE SAME WORD AT THE SAME DEPTH. POST
// /assets/maps/{id}/tiles re-queues a failed build and GET
// .../tiles/{gen}/{z}/{tile} serves one tile of a finished one, so they differ
// by three segments and a method and by nothing else.
//
// THE LAST SEGMENT IS ONE WILDCARD, so the mux checks nothing about what is in
// it. A path with no .webp on the end still matches the pattern and is refused
// by the handler, and this pins that division: the mux decides the shape and
// the handler decides the contents.
func TestMapRoutesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)

	id := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	gen := "01BX5ZZKBKACTAV9WEVGEMMVS3"
	tiles := "/assets/maps/" + id + "/tiles"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/assets/maps", "POST /assets/maps"},
		{http.MethodPost, "/assets/maps/" + id, "POST /assets/maps/{id}"},
		{http.MethodDelete, "/assets/maps/" + id, "DELETE /assets/maps/{id}"},
		{http.MethodPatch, "/assets/maps/" + id + "/name", "PATCH /assets/maps/{id}/name"},
		{http.MethodPost, tiles, "POST /assets/maps/{id}/tiles"},
		{http.MethodGet, tiles + "/" + gen + "/3/2_1.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/" + gen + "/0/23_17.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		// The handler's job, not the mux's.
		{http.MethodGet, tiles + "/" + gen + "/3/2_1", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		{http.MethodGet, tiles + "/not-a-ulid/3/2_1.webp", "GET /assets/maps/{id}/tiles/{gen}/{z}/{tile}"},
		// A wildcard does not match an empty segment, and it does not match two.
		{http.MethodGet, tiles + "/" + gen + "/3/", "/"},
		{http.MethodGet, tiles + "/" + gen + "/3/z/2_1.webp", "/"},
		// A tile is a GET. The retry is the only thing posted under this path,
		// and it is posted three segments higher up.
		{http.MethodPost, tiles + "/" + gen + "/3/2_1.webp", "/"},
		// There is no listing of a map's generations or of its tiles: a
		// renderer computes every URL it needs from five columns on the row.
		{http.MethodGet, tiles, "/"},
		{http.MethodGet, tiles + "/" + gen, "/"},
		// The card's own representation, which a card polls while its tiles
		// are built. A GET returning partial HTML, so it is under /fragment/
		// -- and only a GET: the subtree catch-all takes every other verb,
		// which is what keeps the prefix meaning one thing.
		{http.MethodGet, "/fragment/assets/maps/" + id + "/card", "GET /fragment/assets/maps/{id}/card"},
		{http.MethodPost, "/fragment/assets/maps/" + id + "/card", "/fragment/"},
		{http.MethodGet, "/fragment/assets/maps/" + id, "/fragment/"},
		// The card is a fragment and the tile is not, and they must not be
		// confused: one is markup for a swap and the other is image bytes.
		{http.MethodGet, "/assets/maps/" + id + "/card", "/"},
		// The search box's grid. ONE ROUTE FOR ALL FOUR KINDS, taking the kind
		// as a query parameter rather than a segment -- so it is a literal path
		// and there is no id in it to name somebody else's shelf with. A GET
		// only, like every other fragment: the subtree catch-all takes the rest.
		{http.MethodGet, "/fragment/assets/list", "GET /fragment/assets/list"},
		{http.MethodGet, "/fragment/assets/list?kind=maps&q=keep", "GET /fragment/assets/list"},
		{http.MethodPost, "/fragment/assets/list", "/fragment/"},
		{http.MethodDelete, "/fragment/assets/list", "/fragment/"},
		// The kind is not a segment, so a path shaped like one is not this
		// route -- it falls to the catch-all rather than being served as maps.
		{http.MethodGet, "/fragment/assets/list/maps", "/fragment/"},
		// And it is a fragment, so it does not answer outside the prefix.
		{http.MethodGet, "/assets/list", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}

// THE ASSET MANAGER IS FOUR PAGES AND A REDIRECT ONTO THE FIRST OF THEM, and
// all four are literals. Nothing here is a wildcard, so the mux has nothing to
// disambiguate -- which is exactly why it is worth pinning: the day one of
// these is rewritten as "/assets/{kind}", "images" becomes a kind, every
// avatar and map preview on every page routes to the asset manager instead of
// to its bytes, and every one of them renders as a broken image.
//
// Only maps carries a mutation so far. The other three are a page and nothing
// else, and a POST to one is a miss rather than a pattern that would answer it
// with the page -- which is what a method-less registration would have done.
func TestAssetKindPagesMatchTheirOwnPatterns(t *testing.T) {
	mux := routes(&controllers.App{}, middleware.Auth{}).(*http.ServeMux)

	asset := "01BX5ZZKBKACTAV9WEVGEMMVS2"
	for _, c := range []struct{ method, path, want string }{
		{http.MethodGet, "/assets", "GET /assets"},
		{http.MethodGet, "/assets/maps", "GET /assets/maps"},
		{http.MethodGet, "/assets/tokens", "GET /assets/tokens"},
		{http.MethodGet, "/assets/avatars", "GET /assets/avatars"},
		{http.MethodGet, "/assets/music", "GET /assets/music"},
		// The image proxy sits at the same depth as the four pages, and stays
		// there.
		{http.MethodGet, "/assets/images/" + asset, "GET /assets/images/{id}"},
		{http.MethodGet, "/assets/images/" + asset + "/preview", "GET /assets/images/{id}/preview"},
		// "images" is not a kind and there is no page listing it.
		{http.MethodGet, "/assets/images", "/"},
		// The three kinds that take an upload, each a collection and a member.
		// Tokens and avatars are the same four patterns twice, and they must
		// stay apart: one set of handlers serves both, and the only thing
		// saying which kind a request is for is which pattern it arrived on.
		{http.MethodPost, "/assets/maps", "POST /assets/maps"},
		{http.MethodPost, "/assets/tokens", "POST /assets/tokens"},
		{http.MethodPost, "/assets/tokens/" + asset, "POST /assets/tokens/{id}"},
		{http.MethodPatch, "/assets/tokens/" + asset + "/name", "PATCH /assets/tokens/{id}/name"},
		{http.MethodDelete, "/assets/tokens/" + asset, "DELETE /assets/tokens/{id}"},
		{http.MethodPost, "/assets/avatars", "POST /assets/avatars"},
		{http.MethodPost, "/assets/avatars/" + asset, "POST /assets/avatars/{id}"},
		{http.MethodPatch, "/assets/avatars/" + asset + "/name", "PATCH /assets/avatars/{id}/name"},
		{http.MethodDelete, "/assets/avatars/" + asset, "DELETE /assets/avatars/{id}"},
		// THE COLLECTION AND THE MEMBER DIFFER BY ONE SEGMENT, which is the
		// trap the inventory and spell pairs set too: an upload arriving at the
		// replace handler would parse no id, and a replace arriving at the
		// upload handler would write a second row for a picture that already
		// had one.
		{http.MethodDelete, "/assets/tokens", "/"},
		{http.MethodPatch, "/assets/tokens/" + asset, "/"},
		// A library asset has no representation of its own to GET: its card is
		// rendered by the page, and its bytes come from /assets/images/{id}.
		{http.MethodGet, "/assets/tokens/" + asset, "/"},
		{http.MethodGet, "/assets/avatars/" + asset, "/"},
		// MUSIC, WHOSE UPLOAD IS TWO REQUESTS. The begin is the collection and
		// the confirm hangs off the member, so they differ by two segments; the
		// bytes go to R2 in between and never touch a route here.
		//
		// "confirm" and "audio" are literals in the third segment where no
		// wildcard sits, so neither can be taken for an id.
		{http.MethodPost, "/assets/music", "POST /assets/music"},
		{http.MethodPost, "/assets/music/" + asset + "/confirm", "POST /assets/music/{id}/confirm"},
		{http.MethodPatch, "/assets/music/" + asset + "/name", "PATCH /assets/music/{id}/name"},
		{http.MethodDelete, "/assets/music/" + asset, "DELETE /assets/music/{id}"},
		{http.MethodGet, "/assets/music/" + asset + "/audio", "GET /assets/music/{id}/audio"},
		// There is no replace: a track is deleted and uploaded again, because
		// overwriting one is a second presigned round trip for no gain.
		{http.MethodPost, "/assets/music/" + asset, "/"},
		// The confirm and the player are each one method only. A GET of the
		// confirm would be a mutation behind a link, and a POST to the player
		// is nothing at all.
		{http.MethodGet, "/assets/music/" + asset + "/confirm", "/"},
		{http.MethodPost, "/assets/music/" + asset + "/audio", "/"},
		// No representation of a track to fetch: its card is rendered by the
		// page and its bytes come from the bucket.
		{http.MethodGet, "/assets/music/" + asset, "/"},
		// A kind that is not one of the four. There is no wildcard to catch it,
		// so it falls to the root the way any other unknown path does.
		{http.MethodGet, "/assets/handouts", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}
