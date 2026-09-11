package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) CharacterShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, err := ulid.Parse(r.URL.Query().Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data, err := a.characterShareDialog(ctx, r, characterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load character share", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) CreateCharacterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	if !parsePanelForm(w, r, pages.ShareDialogPanel) {
		return
	}
	input, problems := buildShareInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.ShareDialogPanel, problems)
		return
	}
	token, err := share.NewToken()
	if err != nil {
		slog.Error("Failed to mint a share token", "error", err)
		htmx.ServerError(w)
		return
	}
	params := queries.InsertCharacterShareParams{
		ID:          ulid.Make(),
		Token:       token,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	}
	if input.Password != "" {
		hash, err := share.HashPassword(input.Password)
		if err != nil {
			slog.Error("Failed to hash a share password", "error", err)
			htmx.ServerError(w)
			return
		}
		params.PasswordHash = sql.NullString{String: hash, Valid: true}
	}
	if input.Days > 0 {
		params.ExpiresAt = sql.NullTime{
			Time:  time.Now().Add(time.Duration(input.Days) * 24 * time.Hour),
			Valid: true,
		}
	}
	if _, err := a.Queries.InsertCharacterShare(ctx, params); err != nil {
		if data, readErr := a.characterShareDialog(ctx, r, characterID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}
		slog.Error("Failed to create character share", "error", err)
		htmx.ServerError(w)
		return
	}
	data, err := a.characterShareDialog(ctx, r, characterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load character share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "character")
		return
	}
	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) RevokeCharacterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	result, err := a.Queries.DeleteCharacterShare(ctx, queries.DeleteCharacterShareParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke character share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}
	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(characterShareDialogData(characterID)))
}
func characterShareDialogData(characterID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this character",
		Blurb: "Anyone with the link can read this character sheet, and it stays in step with the sheet as you edit it. " +
			"Journal entries are not included, and neither is the full inventory or spellbook -- only what is equipped and prepared.",
		Action: "/characters/" + characterID.String() + "/share",
	}
}
func (a *App) characterShareDialog(ctx context.Context, r *http.Request, characterID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := characterShareDialogData(characterID)
	row, err := a.Queries.GetCharacterShare(ctx, queries.GetCharacterShareParams{
		CharacterID: characterID,
		OwnerID:     ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	return describeShare(ctx, r, data, row), nil
}
