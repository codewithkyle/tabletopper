package controllers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var errImportedMonsterGone = errors.New("the shared monster no longer exists")

func (a *App) sharedMonster(w http.ResponseWriter, r *http.Request, token string, grant queries.GetShareByTokenRow) {
	ctx := r.Context()
	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      grant.ResourceID,
		OwnerID: grant.OwnerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to load shared monster", "error", err)
		redirectToError(w, r)
		return
	}
	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load shared monster actions", "error", err)
		redirectToError(w, r)
		return
	}
	block := monsterStatBlock(monster, actions, monsterDerived(monster, actions))
	block.Image = ""
	if monster.AssetID != nil {
		block.Image = sharePortraitURL(token)
	}
	shareHeaders(w)
	render(w, r, pages.SharedMonsterPage(pages.SharedMonsterData{
		Block:       block,
		Description: strings.TrimSpace(monster.Description),
		Actions:     sharedMonsterActions(session.FromContext(ctx).UserID, token, grant.OwnerID),
	}))
}
func sharedMonsterActions(viewer ulid.ULID, token string, ownerID ulid.ULID) pages.SharedActions {
	actions := pages.SharedActions{Export: shareExportURL(token)}
	switch {
	case viewer.IsZero():
		actions.Blurb = "Sign in to add this monster to your own manual, as a full copy you can run and edit -- or export it as Markdown for your own notes."
		actions.SignIn = "/sign-in"
	case viewer == ownerID:
		actions.Blurb = "This monster is yours. Export it as Markdown for your own notes."
	default:
		actions.Blurb = "Add this monster to your own manual and you get a full copy to run and edit however you like -- or export it as Markdown for your own notes."
		actions.Import = "/share/" + token + "/import"
	}
	return actions
}
func (a *App) ImportSharedMonster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
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
		slog.Error("Failed to load share for import", "error", err)
		redirectToError(w, r)
		return
	}
	if grant.PasswordHash.Valid && !share.Unlocked(r, token, grant.PasswordHash.String) {
		shareUnavailable(w, r)
		return
	}
	if grant.ResourceType != queries.SharesResourceTypeMonster {
		shareUnavailable(w, r)
		return
	}
	if sess.UserID == grant.OwnerID {
		redirect(w, r, "/monsters/"+grant.ResourceID.String()+"/edit")
		return
	}
	monsterID, err := a.importMonster(ctx, grant, sess.UserID)
	if errors.Is(err, errImportedMonsterGone) {
		shareUnavailable(w, r)
		return
	}
	if err != nil {
		slog.Error("Failed to import a shared monster", "error", err, "monsterID", grant.ResourceID.String())
		redirectToError(w, r)
		return
	}
	if err := a.copyMonsterPicture(ctx, grant, sess.UserID, monsterID); err != nil {
		slog.Error("Failed to copy an imported monster's picture", "error", err, "monsterID", monsterID.String())
	}
	redirect(w, r, "/monsters/"+monsterID.String()+"/edit")
}
func (a *App) importMonster(ctx context.Context, grant queries.GetShareByTokenRow, ownerID ulid.ULID) (ulid.ULID, error) {
	monsterID := ulid.Make()
	err := a.tx(ctx, func(q *queries.Queries) error {
		return copyMonster(ctx, q, grant, ownerID, monsterID)
	})
	if err != nil {
		return ulid.ULID{}, err
	}
	return monsterID, nil
}
func copyMonster(ctx context.Context, q *queries.Queries, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	result, err := q.CopyMonster(ctx, queries.CopyMonsterParams{
		ID:            monsterID,
		OwnerID:       ownerID,
		SourceID:      grant.ResourceID,
		SourceOwnerID: grant.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("monster: %w", err)
	}
	if copied, err := result.RowsAffected(); err == nil && copied == 0 {
		return errImportedMonsterGone
	}
	if err := copyMonsterActions(ctx, q, grant, ownerID, monsterID); err != nil {
		return fmt.Errorf("actions: %w", err)
	}
	return nil
}
func copyMonsterActions(ctx context.Context, q *queries.Queries, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	rows, err := q.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("reading the original: %w", err)
	}
	entropy := ulid.Monotonic(rand.Reader, 0)
	for _, row := range rows {
		id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
		if err != nil {
			return fmt.Errorf("minting an id: %w", err)
		}
		err = q.CopyMonsterAction(ctx, queries.CopyMonsterActionParams{
			ID:          id,
			OwnerID:     ownerID,
			MonsterID:   monsterID,
			Kind:        row.Kind,
			Name:        row.Name,
			Description: row.Description,
		})
		if err != nil {
			return fmt.Errorf("writing the copy: %w", err)
		}
	}
	return nil
}
func (a *App) copyMonsterPicture(ctx context.Context, grant queries.GetShareByTokenRow, ownerID, monsterID ulid.ULID) error {
	picture, err := a.Queries.GetSharedMonsterImage(ctx, queries.GetSharedMonsterImageParams{
		MonsterID: grant.ResourceID,
		OwnerID:   grant.OwnerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the original's asset row: %w", err)
	}
	body, _, err := a.Storage.Get(ctx, picture.FilePath)
	if err != nil {
		return fmt.Errorf("reading the object: %w", err)
	}
	defer body.Close()
	encoded, err := io.ReadAll(io.LimitReader(body, maxUploadBytes))
	if err != nil {
		return fmt.Errorf("reading the object: %w", err)
	}
	if _, err := a.attachMonsterImage(ctx, ownerID, monsterID, encoded, picture.FileName); err != nil {
		return fmt.Errorf("attaching the copy: %w", err)
	}
	return nil
}
