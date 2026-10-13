package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/clerkauth"
	"tabletopper/internal/config"
	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/internal/share"

	"github.com/a-h/templ"
	"github.com/oklog/ulid/v2"
)

type App struct {
	Queries          *queries.Queries
	Storage          Objects
	Clerk            *clerkauth.Client
	Sessions         *session.Store
	Config           config.Config
	Hub              Hub
	DB               *sql.DB
	ShareAttempts    *share.Attempts
	RoomJoinAttempts *share.Attempts
	Purger           Purger
}
type Purger interface {
	Wake()
}
type Objects interface {
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Peek(ctx context.Context, key string, n int64) ([]byte, error)
	Size(ctx context.Context, key string) (int64, error)
	PresignPut(ctx context.Context, key string, contentType string, size int64, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	UploadImage(ctx context.Context, key string, body []byte) error
	UploadCharacterPortrait(ctx context.Context, userID, assetID ulid.ULID, body []byte) error
	UploadJournalImage(ctx context.Context, userID, assetID ulid.ULID, body []byte) error
	UploadMonsterImage(ctx context.Context, userID, assetID ulid.ULID, body []byte) error
	UploadMapOriginal(ctx context.Context, userID, assetID ulid.ULID, body io.ReadSeeker, size int64, contentType string) error
}
type Hub interface {
	Dispatch(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd room.Command) error
	Notify(roomID ulid.ULID, cmd room.Command)
	SyncCharacter(ctx context.Context, roomID, owner, character ulid.ULID)
	Close(ctx context.Context, roomID ulid.ULID)
	ForgetCharacter(ctx context.Context, character ulid.ULID)
	ForgetMonster(ctx context.Context, monster ulid.ULID)
	ForgetAsset(ctx context.Context, asset ulid.ULID)
	Table(ctx context.Context, roomID ulid.ULID) (*hub.TableView, bool)
	Players(ctx context.Context, roomID ulid.ULID) ([]room.Player, bool)
	Pawn(ctx context.Context, roomID, pawnID ulid.ULID, role room.Role) (*room.Pawn, bool)
	CharacterPawn(ctx context.Context, roomID, character ulid.ULID) (*room.Pawn, bool)
	Initiative(ctx context.Context, roomID ulid.ULID, role room.Role) (*hub.InitiativeView, bool)
	Music(ctx context.Context, roomID ulid.ULID) (*hub.MusicView, bool)
	Rolls(ctx context.Context, roomID, viewer ulid.ULID) (*hub.RollsView, bool)
	Export(ctx context.Context, roomID ulid.ULID) (*hub.ExportView, bool)
	Debug(ctx context.Context, roomID ulid.ULID) (*hub.DebugView, bool)
	Autosave(ctx context.Context, roomID ulid.ULID) bool
	Serve(w http.ResponseWriter, r *http.Request, roomID ulid.ULID, p room.Player, member hub.Membership)
	Version() string
}

func (a *App) tx(ctx context.Context, fn func(q *queries.Queries) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(a.Queries.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("Failed to render", "path", r.URL.Path, "error", err)
	}
}
func renderFragment(w http.ResponseWriter, r *http.Request, c templ.Component) {
	renderStatus(w, r, http.StatusOK, c)
}
func renderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, c)
}
func pathID(w http.ResponseWriter, r *http.Request, name, what string) (ulid.ULID, bool) {
	id, err := ulid.Parse(r.PathValue(name))
	if err != nil {
		htmx.NotFound(w, what)
		return ulid.ULID{}, false
	}
	return id, true
}
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func redirectToError(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/error")
}
