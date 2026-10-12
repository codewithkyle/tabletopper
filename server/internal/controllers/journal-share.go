package controllers

import (
	"context"
	"database/sql"
	"net/http"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

var journalShare = shareKind{
	what:    "journal entry",
	heading: "Share this entry",
	blurb:   "Anyone with the link can read this entry. It stays in step with the entry, so later edits show up for them too.",
	action: func(s shareSubject) string {
		return "/characters/" + s.id.String() + "/journal/" + s.entry.String() + "/share"
	},
	fromQuery: func(r *http.Request) (shareSubject, bool) {
		params := r.URL.Query()
		id, err := ulid.Parse(params.Get("character"))
		if err != nil {
			return shareSubject{}, false
		}
		entry, err := ulid.Parse(params.Get("entry"))
		return shareSubject{id: id, entry: entry}, err == nil
	},
	fromPath: func(w http.ResponseWriter, r *http.Request) (shareSubject, bool) {
		id, ok := panelCharacterID(w, r)
		if !ok {
			return shareSubject{}, false
		}
		entry, ok := journalEntryID(w, r)
		return shareSubject{id: id, entry: entry}, ok
	},
	get: func(ctx context.Context, q *queries.Queries, s shareSubject) (queries.Share, error) {
		return q.GetJournalShare(ctx, queries.GetJournalShareParams{EntryID: s.entry, CharacterID: &s.id, OwnerID: s.owner})
	},
	insert: func(ctx context.Context, q *queries.Queries, s shareSubject, g shareGrant) error {
		_, err := q.InsertJournalShare(ctx, queries.InsertJournalShareParams{
			ID:           g.id,
			Token:        g.token,
			PasswordHash: g.password,
			ExpiresAt:    g.expires,
			EntryID:      s.entry,
			CharacterID:  s.id,
			OwnerID:      s.owner,
		})
		return err
	},
	delete: func(ctx context.Context, q *queries.Queries, s shareSubject) (sql.Result, error) {
		return q.DeleteJournalShare(ctx, queries.DeleteJournalShareParams{EntryID: s.entry, CharacterID: &s.id, OwnerID: s.owner})
	},
}

func (a *App) JournalShareFragment(w http.ResponseWriter, r *http.Request) {
	a.shareFragment(w, r, journalShare)
}
func (a *App) CreateJournalShare(w http.ResponseWriter, r *http.Request) {
	a.createShare(w, r, journalShare)
}
func (a *App) RevokeJournalShare(w http.ResponseWriter, r *http.Request) {
	a.revokeShare(w, r, journalShare)
}
