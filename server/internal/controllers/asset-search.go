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





















const mapsSlug = "maps"


















func (a *App) AssetListFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	
	
	
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
