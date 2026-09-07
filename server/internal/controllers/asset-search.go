package controllers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/a-h/templ"
	"github.com/oklog/ulid/v2"
)

// THE FOUR MANAGER PAGES SHARE ONE SEARCH ROUTE, and the kind is a query
// parameter here where it is a path segment everywhere else in the manager.
//
// THAT LOOKS LIKE AN INCONSISTENCY AND IS NOT. The pages are four literal
// routes because their handlers do not resemble each other -- a map is tiled by
// a background worker, music is not an image at all. This one is the opposite
// case: the handler is genuinely the same work four times, and the only thing
// the kind decides is which statement to run and which component to render. A
// route per kind here would be four registrations pointing at four one-line
// wrappers around this function.
//
// THE VALUE COMES OFF THE WIRE, SO IT IS MATCHED AGAINST A KNOWN SET BEFORE
// ANYTHING IS QUERIED. That is the fragment rule and it is also what keeps the
// type parameter on SearchLibraryAssets safe: the switch below is the only
// thing that ever chooses one, and it can only choose from four.

// mapsSlug is the segment maps are routed under, which is also the id of their
// grid and the value their search box sends. The other three kinds carry theirs
// on an assetKind; maps have no descriptor because they share none of those
// handlers, so this is the one place the word has to be written down.
const mapsSlug = "maps"

// AssetListFragment is the grid under one manager page's search box, filtered
// by ?q=. It is a GET returning the same component the page renders inside its
// shell, which is what the /fragment/ prefix promises.
//
// IT LOADS NOTHING TO CHECK OWNERSHIP, because owner_id is in every statement
// beside the term: there is no id in this URL, so somebody else's shelf is not
// addressable from here at all.
//
// THE SEARCH IS NOT PUSHED INTO THE URL, and hx-push-url is deliberately absent
// from the box, for the reason the journal's search gives: htmx pushes on every
// swap and the swaps are on a debounce, so pushing would file a history entry
// per pause in typing.
//
// An overlong term and an unknown kind are both a 404 with an empty body rather
// than an alert. The box carries a maxlength and the kind is baked into its
// hx-get, so neither can come from the page -- there is nobody on the other end
// to tell.
func (a *App) AssetListFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	// Trimmed, so a term somebody is still typing a space into does not stop
	// matching, and so a box holding nothing but spaces is the whole shelf
	// rather than a search for a space.
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cards, err := a.assetCards(ctx, sess.UserID, r.URL.Query().Get("kind"), term)
	if err != nil {
		slog.Error("Failed to search assets", "error", err, "kind", r.URL.Query().Get("kind"))
		htmx.ServerError(w)
		return
	}
	if cards == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, cards)
}

// assetCards is the switch: one kind in, that kind's filtered grid out. A nil
// component and a nil error is a kind that is not one of the four, which the
// caller answers with an empty 404 -- an error would be a lie, because nothing
// went wrong.
func (a *App) assetCards(ctx context.Context, ownerID ulid.ULID, kind string, term string) (templ.Component, error) {
	switch kind {
	case mapsSlug:
		cards, err := a.mapList(ctx, ownerID, term)
		if err != nil {
			return nil, err
		}
		return pages.MapCards(cards, term), nil

	case tokenKind.Slug:
		return a.libraryCards(ctx, ownerID, tokenKind, term)

	case avatarKind.Slug:
		return a.libraryCards(ctx, ownerID, avatarKind, term)

	case musicKind.Slug:
		tracks, err := a.musicList(ctx, ownerID, term)
		if err != nil {
			return nil, err
		}
		return pages.MusicCards(tracks, term), nil
	}

	return nil, nil
}

func (a *App) libraryCards(ctx context.Context, ownerID ulid.ULID, kind libraryKind, term string) (templ.Component, error) {
	cards, err := a.libraryList(ctx, ownerID, kind, term)
	if err != nil {
		return nil, err
	}

	return kind.Cards(cards, term), nil
}
