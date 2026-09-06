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
		{http.MethodGet, "/fragment/character/new", "GET /fragment/character/new"},
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
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}
