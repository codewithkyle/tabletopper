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
func (a *App) JournalShareFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	params := r.URL.Query()
	characterID, err := ulid.Parse(params.Get("character"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	entryID, err := ulid.Parse(params.Get("entry"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data, err := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load journal share", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) CreateJournalShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
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
	params := queries.InsertJournalShareParams{
		ID:          ulid.Make(),
		Token:       token,
		EntryID:     entryID,
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
	if _, err := a.Queries.InsertJournalShare(ctx, params); err != nil {
		if data, readErr := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID); readErr == nil && data.Link != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			render(w, r, pages.ShareDialog(data))
			return
		}
		slog.Error("Failed to create journal share", "error", err)
		htmx.ServerError(w)
		return
	}
	data, err := a.journalShareDialog(ctx, r, characterID, entryID, sess.UserID)
	if err != nil {
		slog.Error("Failed to load journal share after creating it", "error", err)
		htmx.ServerError(w)
		return
	}
	if data.Link == "" {
		htmx.NotFound(w, "journal entry")
		return
	}
	htmx.Toast(w, "Share link created.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(data))
}
func (a *App) RevokeJournalShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
	if !ok {
		return
	}
	result, err := a.Queries.DeleteJournalShare(ctx, queries.DeleteJournalShareParams{
		EntryID:     entryID,
		CharacterID: &characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to revoke journal share", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "share link")
		return
	}
	htmx.Toast(w, "Share link revoked.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.ShareDialog(journalShareDialogData(characterID, entryID)))
}
func journalShareDialogData(characterID, entryID ulid.ULID) pages.ShareDialogData {
	return pages.ShareDialogData{
		Heading: "Share this entry",
		Blurb:   "Anyone with the link can read this entry. It stays in step with the entry, so later edits show up for them too.",
		Action:  "/characters/" + characterID.String() + "/journal/" + entryID.String() + "/share",
	}
}
func (a *App) journalShareDialog(ctx context.Context, r *http.Request, characterID, entryID, ownerID ulid.ULID) (pages.ShareDialogData, error) {
	data := journalShareDialogData(characterID, entryID)
	row, err := a.Queries.GetJournalShare(ctx, queries.GetJournalShareParams{
		EntryID:     entryID,
		CharacterID: &characterID,
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
