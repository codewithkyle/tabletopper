package controllers

import (
	"context"
	"database/sql"
	"net/http"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

var monsterShare = shareKind{
	what:    "monster",
	heading: "Share this monster",
	blurb: "Anyone with the link can read this monster's full stat block, and it stays in step with it as you edit. " +
		"Anyone signed in can also copy it into their own manual, and a copy is theirs -- revoking the link later does not take it back.",
	action: func(s shareSubject) string { return "/monsters/" + s.id.String() + "/share" },
	fromQuery: func(r *http.Request) (shareSubject, bool) {
		id, err := ulid.Parse(r.URL.Query().Get("monster"))
		return shareSubject{id: id}, err == nil
	},
	fromPath: func(w http.ResponseWriter, r *http.Request) (shareSubject, bool) {
		id, ok := panelMonsterID(w, r)
		return shareSubject{id: id}, ok
	},
	get: func(ctx context.Context, q *queries.Queries, s shareSubject) (queries.Share, error) {
		return q.GetMonsterShare(ctx, queries.GetMonsterShareParams{MonsterID: s.id, OwnerID: s.owner})
	},
	insert: func(ctx context.Context, q *queries.Queries, s shareSubject, g shareGrant) error {
		_, err := q.InsertMonsterShare(ctx, queries.InsertMonsterShareParams{
			ID:           g.id,
			Token:        g.token,
			PasswordHash: g.password,
			ExpiresAt:    g.expires,
			MonsterID:    s.id,
			OwnerID:      s.owner,
		})
		return err
	},
	delete: func(ctx context.Context, q *queries.Queries, s shareSubject) (sql.Result, error) {
		return q.DeleteMonsterShare(ctx, queries.DeleteMonsterShareParams{MonsterID: s.id, OwnerID: s.owner})
	},
}

func (a *App) MonsterShareFragment(w http.ResponseWriter, r *http.Request) {
	a.shareFragment(w, r, monsterShare)
}
func (a *App) CreateMonsterShare(w http.ResponseWriter, r *http.Request) {
	a.createShare(w, r, monsterShare)
}
func (a *App) RevokeMonsterShare(w http.ResponseWriter, r *http.Request) {
	a.revokeShare(w, r, monsterShare)
}
