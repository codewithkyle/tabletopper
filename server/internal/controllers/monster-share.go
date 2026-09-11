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

func (a *App) MonsterShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, err := ulid.Parse(r.URL.Query().Get("monster"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data, err := a.monsterShareDialog(ctx, r, monsterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load monster share", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) CreateMonsterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
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
	params := queries.InsertMonsterShareParams{
		ID:        ulid.Make(),
		Token:     token,
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
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
	if _, err := a.Queries.InsertMonsterShare(ctx, params); err != nil {
		if data, readErr := a.monsterShareDialog(ctx, r, monsterID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}
		slog.Error("Failed to create monster share", "error", err)
		htmx.ServerError(w)
		return
	}
	data, err := a.monsterShareDialog(ctx, r, monsterID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load monster share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "monster")
		return
	}
	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) RevokeMonsterShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, ok := panelMonsterID(w, r)
	if !ok {
		return
	}
	result, err := a.Queries.DeleteMonsterShare(ctx, queries.DeleteMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke monster share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}
	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(monsterShareDialogData(monsterID)))
}
func monsterShareDialogData(monsterID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this monster",
		Blurb: "Anyone with the link can read this monster's full stat block, and it stays in step with it as you edit. " +
			"Anyone signed in can also copy it into their own manual, and a copy is theirs -- revoking the link later does not take it back.",
		Action: "/monsters/" + monsterID.String() + "/share",
	}
}
func (a *App) monsterShareDialog(ctx context.Context, r *http.Request, monsterID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := monsterShareDialogData(monsterID)
	row, err := a.Queries.GetMonsterShare(ctx, queries.GetMonsterShareParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	return describeShare(ctx, r, data, row), nil
}
