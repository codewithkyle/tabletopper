package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tabletopper/internal/markdown"
	"tabletopper/internal/queries"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) SharePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")
	if !share.ValidToken(token) {
		shareUnavailable(w, r)
		return
	}
	grant, err := a.Queries.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load share", "error", err)
		redirectToError(w, r)
		return
	}
	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareHeaders(w)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{Action: "/share/" + token}))
		return
	}
	switch grant.ResourceType {
	case queries.SharesResourceTypeJournal:
		a.sharedJournalEntry(w, r, token, grant)
	case queries.SharesResourceTypeCharacter:
		a.sharedCharacterSheet(w, r, token, grant)
	case queries.SharesResourceTypeMonster:
		a.sharedMonster(w, r, token, grant)
	default:
		slog.Error("Share names a resource type this build cannot render", "type", grant.ResourceType)
		shareUnavailable(w, r)
	}
}
func (a *App) sharedJournalEntry(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	ctx := r.Context()
	characterID, ok := shareCharacter(grant)
	if !ok {
		slog.Error("A journal share names no character", "shareID", grant.ID.String())
		shareUnavailable(w, r)
		return
	}
	entry, err := a.Queries.GetSharedJournalEntry(ctx, queries.GetSharedJournalEntryParams{
		EntryID:     grant.ResourceID,
		CharacterID: characterID,
		OwnerID:     grant.OwnerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared journal entry", "error", err)
		redirectToError(w, r)
		return
	}
	body, err := markdown.Render(entry.Body, shareImageSource(token, characterID, grant.ResourceID))
	if err != nil {
		slog.Error("Failed to render shared journal entry", "error", err)
		redirectToError(w, r)
		return
	}
	character := pages.SharedCharacter{
		Name:    entry.Name,
		Level:   strconv.Itoa(int(entry.Level)),
		Classes: fallbackString(nullStringValue(entry.Classes), "Class Unknown"),
		Race:    fallbackString(nullStringValue(entry.Race), "Unknown Lineage"),
	}
	if entry.AssetID != nil {
		character.Avatar = sharePortraitURL(token)
	}
	shareHeaders(w)
	render(w, r, pages.SharedJournalEntry(pages.SharedJournalData{
		Character: character,
		Title:     entry.Title,
		Body:      body,
	}))
}
func (a *App) UnlockShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")
	if !share.ValidToken(token) {
		shareUnavailable(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		shareUnavailable(w, r)
		return
	}
	grant, err := a.Queries.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load share for unlock", "error", err)
		redirectToError(w, r)
		return
	}
	if !grant.PasswordHash.Valid {
		redirect(w, r, "/share/"+token)
		return
	}
	if !a.ShareAttempts.Allow(token, time.Now()) {
		shareHeaders(w)
		w.WriteHeader(http.StatusTooManyRequests)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{
			Action:  "/share/" + token,
			Problem: "Too many attempts. Wait a minute and try again.",
		}))
		return
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ok, err := share.PasswordMatches(checkCtx, grant.PasswordHash.String, r.PostFormValue("password"))
	if err != nil {
		slog.Warn("Gave up waiting to check a share password", "error", err)
		shareHeaders(w)
		w.WriteHeader(http.StatusServiceUnavailable)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{
			Action:  "/share/" + token,
			Problem: "The server is busy. Try again in a moment.",
		}))
		return
	}
	if !ok {
		shareHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, pages.ShareLocked(pages.ShareLockedData{
			Action:  "/share/" + token,
			Problem: "That password is not right.",
		}))
		return
	}
	share.SetUnlocked(w, token, grant.PasswordHash.String, !a.Config.Development())
	redirect(w, r, "/share/"+token)
}
func (a *App) GetShareImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")
	assetID, err := ulid.Parse(r.PathValue("assetId"))
	if !share.ValidToken(token) || err != nil {
		http.NotFound(w, r)
		return
	}
	grant, ok := a.shareGrant(w, r, token)
	if !ok {
		return
	}
	if grant.ResourceType != queries.SharesResourceTypeJournal {
		http.NotFound(w, r)
		return
	}
	characterID, ok := shareCharacter(grant)
	if !ok {
		http.NotFound(w, r)
		return
	}
	key, err := a.Queries.GetJournalImage(ctx, queries.GetJournalImageParams{
		AssetID:     assetID,
		EntryID:     grant.ResourceID,
		CharacterID: characterID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load shared journal image row", "error", err, "assetID", assetID.String())
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	a.streamImage(w, r, key, `"`+assetID.String()+`"`)
}
func (a *App) GetSharePortrait(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !share.ValidToken(token) {
		http.NotFound(w, r)
		return
	}
	grant, ok := a.shareGrant(w, r, token)
	if !ok {
		return
	}
	switch grant.ResourceType {
	case queries.SharesResourceTypeJournal, queries.SharesResourceTypeCharacter:
		a.sharedCharacterPortrait(w, r, grant)
	case queries.SharesResourceTypeMonster:
		a.sharedMonsterPortrait(w, r, grant)
	default:
		http.NotFound(w, r)
	}
}
func (a *App) sharedCharacterPortrait(w http.ResponseWriter, r *http.Request, grant queries.GetShareByTokenRow) {
	characterID, ok := shareCharacter(grant)
	if !ok {
		http.NotFound(w, r)
		return
	}
	avatar, err := a.Queries.GetSharedCharacterAvatar(r.Context(), queries.GetSharedCharacterAvatarParams{
		CharacterID: characterID,
		OwnerID:     grant.OwnerID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load shared character avatar", "error", err, "characterID", characterID.String())
		}
		http.NotFound(w, r)
		return
	}
	sharePortrait(w, r, a, avatar.FilePath, characterID, avatar.UpdatedAt)
}
func (a *App) sharedMonsterPortrait(w http.ResponseWriter, r *http.Request, grant queries.GetShareByTokenRow) {
	picture, err := a.Queries.GetSharedMonsterImage(r.Context(), queries.GetSharedMonsterImageParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load shared monster image", "error", err, "monsterID", grant.ResourceID.String())
		}
		http.NotFound(w, r)
		return
	}
	sharePortrait(w, r, a, picture.FilePath, grant.ResourceID, picture.UpdatedAt)
}
func sharePortrait(w http.ResponseWriter, r *http.Request, a *App, key string, id ulid.ULID, updated time.Time) {
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	a.streamImage(w, r, key, fmt.Sprintf(`"%s-%d"`, id, updated.Unix()))
}
func shareCharacter(grant queries.GetShareByTokenRow) (ulid.ULID, bool) {
	if grant.CharacterID == nil {
		return ulid.ULID{}, false
	}
	return *grant.CharacterID, true
}
func (a *App) shareGrant(w http.ResponseWriter, r *http.Request, token string) (queries.GetShareByTokenRow, bool) {
	grant, err := a.Queries.GetShareByToken(r.Context(), token)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load share", "error", err)
		}
		http.NotFound(w, r)
		return queries.GetShareByTokenRow{}, false
	}
	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		http.NotFound(w, r)
		return queries.GetShareByTokenRow{}, false
	}
	return grant, true
}
func sharePortraitURL(token string) string {
	return "/share/" + token + "/portrait"
}
func shareExportURL(token string) string {
	return "/share/" + token + "/export.md"
}
func shareImageSource(token string, characterID, entryID ulid.ULID) markdown.ImageSource {
	prefix := journalImagePrefix(characterID, entryID)
	return func(dest string) (string, bool) {
		assetID, found := strings.CutPrefix(dest, prefix)
		if !found {
			return "", false
		}
		if _, err := ulid.Parse(assetID); err != nil {
			return "", false
		}
		return "/share/" + token + "/images/" + assetID, true
	}
}
func shareHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "img-src 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")
}
func shareUnavailable(w http.ResponseWriter, r *http.Request) {
	shareHeaders(w)
	w.WriteHeader(http.StatusNotFound)
	render(w, r, pages.ShareUnavailable())
}
