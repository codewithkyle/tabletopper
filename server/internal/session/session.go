package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"tabletopper/internal/queries"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	// IdleWindow is how long a session survives without activity. Every request
	// slides it forward, so this is an inactivity timeout rather than a hard one.
	IdleWindow = 7 * 24 * time.Hour

	// MaxLifetime caps a session no matter how active it is, forcing a periodic
	// trip back through Clerk. Refresh clamps to it, so this is the real ceiling
	// measured from created_at rather than an approximate one.
	MaxLifetime = 30 * 24 * time.Hour

	// refreshInterval throttles how often an active session is written back to
	// the DB. Requests arriving inside the interval do no work.
	refreshInterval = time.Hour
)

type UserSession struct {
	ID              ulid.ULID
	UserID          ulid.ULID
	CharacterID     *ulid.ULID
	RoomID          *ulid.ULID
	Username        string
	ProfileImageURL string
	Hash            []byte
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

func New() *UserSession {
	s := UserSession{}
	s.ID = ulid.Make()
	return &s
}

func GetUserSessionFromCookie(r *http.Request, db *sql.DB) (UserSession, error) {
	session := UserSession{}
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return session, err
		}
		slog.Error("Failed to get user session from cookie", "error", err)
		return session, err
	}

	hash, err := DecodeHash(cookie.Value)
	if err != nil {
		slog.Error("Failed to decode hash", "error", err)
		return session, err
	}

	q := queries.New(db)
	result, err := q.GetSession(r.Context(), hash)
	if err != nil {
		slog.Error("Failed to get user session from DB", "error", err)
		return session, err
	}

	// NOTE: a malformed ULID fails in Scan above, so these are already
	// length-checked; a NULL character_id or room_id arrives as a nil pointer
	session.Hash = hash
	session.CreatedAt = result.CreatedAt
	session.ID = result.ID
	session.UserID = result.UserID
	session.Username = result.Username
	session.ProfileImageURL = result.ProfileImageURL
	session.CharacterID = result.CharacterID
	session.RoomID = result.RoomID

	return session, nil
}

func (s *UserSession) CreateSession(db *sql.DB, ctx context.Context) error {
	q := queries.New(db)
	s.Hash = make([]byte, 32)
	if _, err := rand.Read(s.Hash); err != nil {
		return err
	}
	s.ExpiresAt = time.Now().Add(IdleWindow)
	err := q.StartSession(ctx, queries.StartSessionParams{
		ID:              s.ID,
		Username:        s.Username,
		ProfileImageURL: s.ProfileImageURL,
		UserID:          s.UserID,
		ExpiresAt:       s.ExpiresAt,
		Hash:            s.Hash,
	})
	if err != nil {
		return err
	}
	return nil
}

// nextExpiry returns a refreshed session's new expiry: one idle window out,
// clamped to the absolute cap measured from creation. Without the clamp a
// refresh landing just inside MaxLifetime would push expiry a further
// IdleWindow beyond it.
func nextExpiry(now time.Time, createdAt time.Time) time.Time {
	expiresAt := now.Add(IdleWindow)
	if cap := createdAt.Add(MaxLifetime); expiresAt.After(cap) {
		return cap
	}
	return expiresAt
}

// Refresh slides an active session's expiry forward and re-issues the cookie to
// match. It is throttled to one write per refreshInterval and will not extend a
// session past MaxLifetime, so most requests match no rows and do no work.
//
// The cookie has to be re-issued alongside the row: leaving it on its original
// Expires would have the browser drop it while the session was still live.
func (s *UserSession) Refresh(r *http.Request, w http.ResponseWriter, db *sql.DB) error {
	now := time.Now()
	expiresAt := nextExpiry(now, s.CreatedAt)

	q := queries.New(db)
	result, err := q.RefreshSession(r.Context(), queries.RefreshSessionParams{
		ExpiresAt:      expiresAt,
		Hash:           s.Hash,
		RefreshCutoff:  now.Add(-refreshInterval),
		LifetimeCutoff: now.Add(-MaxLifetime),
	})
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// NOTE: throttled, or the session has hit MaxLifetime and is being
		// allowed to run out its remaining window
		return nil
	}

	s.ExpiresAt = expiresAt
	s.SetCookie(w)
	return nil
}

func (s UserSession) EncodeHash() string {
	return base64.RawURLEncoding.EncodeToString(s.Hash)
}

func DecodeHash(hash string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(hash)
	if err != nil {
		return []byte{}, err
	}
	return raw, nil
}

// secureCookies reports whether the session cookie should carry the Secure
// flag. Only an explicit development ENV opts out, so a missing or misspelled
// value fails closed.
func secureCookies() bool {
	switch os.Getenv("ENV") {
	case "development", "dev", "local":
		return false
	default:
		return true
	}
}

func (s UserSession) SetCookie(w http.ResponseWriter) {
	hash := s.EncodeHash()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    hash,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
		Expires:  s.ExpiresAt,
	})
}

func Logout(r *http.Request, w http.ResponseWriter, db *sql.DB) error {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if err == http.ErrNoCookie {
			return nil
		}
		return err
	}

	hash, err := DecodeHash(cookie.Value)
	if err != nil {
		slog.Error("Failed to decode hash", "error", err)
		return err
	}

	q := queries.New(db)
	err = q.EndSession(r.Context(), hash)
	if err != nil {
		slog.Error("Failed to end session in DB", "error", err)
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}
