package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"tabletopper/internal/export"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"

	"github.com/oklog/ulid/v2"
)





























func (a *App) ExportMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/monsters")
		return
	}

	file, name, err := a.monsterMarkdown(ctx, monsterID, sess.UserID)
	
	
	
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/monsters")
		return
	}
	if err != nil {
		slog.Error("Failed to export monster", "error", err, "monsterID", monsterID.String())
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, "monster"))
}




func (a *App) ExportCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/characters")
		return
	}

	file, name, err := a.characterMarkdown(ctx, characterID, sess.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/characters")
		return
	}
	if err != nil {
		slog.Error("Failed to export character", "error", err, "characterID", characterID.String())
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, "character"))
}












func (a *App) ExportShare(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("Failed to load share for export", "error", err)
		redirectToError(w, r)
		return
	}

	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareUnavailable(w, r)
		return
	}

	var file []byte
	var name, fallback string

	switch grant.ResourceType {
	case queries.SharesResourceTypeMonster:
		fallback = "monster"
		file, name, err = a.monsterMarkdown(ctx, grant.ResourceID, grant.OwnerID)
	case queries.SharesResourceTypeCharacter:
		fallback = "character"
		file, name, err = a.characterMarkdown(ctx, grant.ResourceID, grant.OwnerID)
	default:
		shareUnavailable(w, r)
		return
	}

	
	
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to export a shared page", "error", err, "type", grant.ResourceType)
		redirectToError(w, r)
		return
	}

	writeMarkdown(w, file, export.Filename(name, fallback))
}




func (a *App) monsterMarkdown(ctx context.Context, monsterID, ownerID ulid.ULID) ([]byte, string, error) {
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{ID: monsterID, OwnerID: ownerID})
	if err != nil {
		return nil, "", err
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monsterID,
		OwnerID:   ownerID,
	})
	if err != nil {
		return nil, "", err
	}

	block := monsterStatBlock(monster, actions, monsterDerived(monster, actions))

	return export.Monster(block, monster.Description), monster.Name, nil
}












func (a *App) characterMarkdown(ctx context.Context, characterID, ownerID ulid.ULID) ([]byte, string, error) {
	sheet, _, err := a.loadCharacterSheet(ctx, characterID, ownerID)
	if err != nil {
		return nil, "", err
	}

	return export.Character(sheet), sheet.Header.Name, nil
}











func writeMarkdown(w http.ResponseWriter, file []byte, filename string) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	if _, err := w.Write(file); err != nil {
		slog.Error("Failed to write a Markdown export", "error", err)
	}
}
