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
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/characters/" + id + "/avatar", "POST /characters/{id}/avatar"},
		{http.MethodPost, "/characters/" + id + "/identity", "POST /characters/{id}/identity"},
		{http.MethodPost, "/characters/" + id + "/core-stats", "POST /characters/{id}/core-stats"},
		{http.MethodPost, "/characters/" + id + "/spells", "POST /characters/{id}/spells"},
		{http.MethodPost, "/characters/" + id + "/bonuses/skills", "POST /characters/{id}/bonuses/{kind}"},
		{http.MethodPost, "/characters/" + id + "/rows/features", "POST /characters/{id}/rows/{field}"},
		{http.MethodGet, "/characters/" + id + "/edit", "GET /characters/{id}/edit"},
		{http.MethodGet, "/characters/" + id + "/edit/spells", "GET /characters/{id}/edit/spells"},
		{http.MethodGet, "/fragment/character/new", "GET /fragment/character/new"},
		{http.MethodGet, "/fragment/character/spell-card", "GET /fragment/character/spell-card"},
		// None of these is a route any more, so all three fall to the catch-all
		// rather than to one of the above. The first two were the whole-sheet
		// save, which the panels replaced. The third was the create page, which
		// the dialog replaced -- and it is the one most likely to be re-added by
		// accident, because "/characters/new" and "/characters/{id}/edit" look
		// like a matched pair.
		{http.MethodPost, "/characters/" + id + "/rows", "/"},
		{http.MethodPost, "/characters/" + id, "/"},
		{http.MethodGet, "/characters/new", "/"},
	} {
		_, pattern := mux.Handler(httptest.NewRequest(c.method, c.path, nil))
		if pattern != c.want {
			t.Errorf("%s %s matched %q, want %q", c.method, c.path, pattern, c.want)
		}
	}
}
