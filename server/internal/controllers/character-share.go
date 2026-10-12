package controllers

import (
	"context"
	"database/sql"
	"net/http"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

var characterShare = shareKind{
	what:    "character",
	heading: "Share this character",
	blurb: "Anyone with the link can read this whole character sheet, inventory and spells included, and it stays in step with the sheet as you edit it. " +
		"Journal entries are not included.",
	action: func(s shareSubject) string { return "/characters/" + s.id.String() + "/share" },
	fromQuery: func(r *http.Request) (shareSubject, bool) {
		id, err := ulid.Parse(r.URL.Query().Get("character"))
		return shareSubject{id: id}, err == nil
	},
	fromPath: func(w http.ResponseWriter, r *http.Request) (shareSubject, bool) {
		id, ok := panelCharacterID(w, r)
		return shareSubject{id: id}, ok
	},
	get: func(ctx context.Context, q *queries.Queries, s shareSubject) (queries.Share, error) {
		return q.GetCharacterShare(ctx, queries.GetCharacterShareParams{CharacterID: s.id, OwnerID: s.owner})
	},
	insert: func(ctx context.Context, q *queries.Queries, s shareSubject, g shareGrant) error {
		_, err := q.InsertCharacterShare(ctx, queries.InsertCharacterShareParams{
			ID:           g.id,
			Token:        g.token,
			PasswordHash: g.password,
			ExpiresAt:    g.expires,
			CharacterID:  s.id,
			OwnerID:      s.owner,
		})
		return err
	},
	delete: func(ctx context.Context, q *queries.Queries, s shareSubject) (sql.Result, error) {
		return q.DeleteCharacterShare(ctx, queries.DeleteCharacterShareParams{CharacterID: s.id, OwnerID: s.owner})
	},
}

func (a *App) CharacterShareFragment(w http.ResponseWriter, r *http.Request) {
	a.shareFragment(w, r, characterShare)
}
func (a *App) CreateCharacterShare(w http.ResponseWriter, r *http.Request) {
	a.createShare(w, r, characterShare)
}
func (a *App) RevokeCharacterShare(w http.ResponseWriter, r *http.Request) {
	a.revokeShare(w, r, characterShare)
}
