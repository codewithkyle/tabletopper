// Package session is the login session: the row in the sessions table, the
// cookie that names it, and the sliding expiry that ties the two together.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"tabletopper/internal/prefs"
	"tabletopper/internal/queries"

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

	cookieName = "session_id"
)

// UserSession is one login. It is plain data: the Store is what reads and
// writes it.
//
// The browser holds a random token and the row holds its SHA-256, so a read
// of the sessions table cannot mint a cookie. Hash is the stored side and is
// what every query keys on; the token itself lives only in the cookie and,
// while a request is in flight, in the unexported field below.
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

	// Prefs is the account settings the page renders with: theme, zone, date
	// order and clock.
	//
	// IT IS JOINED, NOT COPIED INTO THE ROW. Username and ProfileImageURL above
	// are copies on the sessions row, and that is right for them -- both come
	// from Clerk and only change when a login refreshes them. These change
	// while the user is sitting in the app, and one user has several sessions,
	// so a copy would mean switching to dark on a laptop and watching the phone
	// stay light until its session expired a week later.
	//
	// The join is to users on its primary key, inside a lookup that already
	// runs on every request. It is the cheapest correct answer, and there is no
	// second write to keep in step with the first.
	Prefs prefs.Preferences

	// Onboarded is false while this account has never answered the welcome
	// dialog, which is what opens it on the homepage.
	//
	// It is the boolean and not the timestamp because nothing renders "when".
	// The column holds the instant, and this is the only question anything asks
	// of it -- carrying the time as well would put a nullable field on the
	// session that every reader would have to remember not to use.
	Onboarded bool

	token []byte
}

// Store reads and writes sessions and the cookie that names them. It is the
// only code that touches either, so the row and the cookie cannot drift.
type Store struct {
	q      *queries.Queries
	secure bool
}

// NewStore returns a Store over q. secure sets the cookie's Secure flag and
// should be false only for local development over plain HTTP.
func NewStore(q *queries.Queries, secure bool) *Store {
	return &Store{q: q, secure: secure}
}

// FromRequest loads the session the request's cookie names. A request with
// no cookie returns http.ErrNoCookie before the database is touched.
func (s *Store) FromRequest(r *http.Request) (UserSession, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return UserSession{}, err
	}

	token, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return UserSession{}, fmt.Errorf("session: decode cookie: %w", err)
	}
	hash := hashToken(token)

	row, err := s.q.GetSession(r.Context(), hash)
	if err != nil {
		return UserSession{}, fmt.Errorf("session: load: %w", err)
	}

	// NOTE: a malformed ULID fails in Scan above, so these are already
	// length-checked; a NULL character_id or room_id arrives as a nil pointer
	return UserSession{
		ID:              row.ID,
		UserID:          row.UserID,
		CharacterID:     row.CharacterID,
		RoomID:          row.RoomID,
		Username:        row.Username,
		ProfileImageURL: row.ProfileImageURL,
		Hash:            hash,
		CreatedAt:       row.CreatedAt,
		Prefs: prefs.New(
			string(row.Theme),
			row.Timezone,
			string(row.DateFormat),
			string(row.TimeFormat),
		),
		Onboarded: row.OnboardedAt.Valid,
		token:     token,
	}, nil
}

// Create starts a session for u and sets its cookie. u carries the user in:
// UserID, Username and ProfileImageURL must be set. The ID, token and
// timestamps are filled in here.
func (s *Store) Create(ctx context.Context, w http.ResponseWriter, u *UserSession) error {
	u.ID = ulid.Make()
	u.token = make([]byte, 32)
	if _, err := rand.Read(u.token); err != nil {
		return fmt.Errorf("session: token: %w", err)
	}
	u.Hash = hashToken(u.token)
	u.CreatedAt = time.Now()
	u.ExpiresAt = u.CreatedAt.Add(IdleWindow)

	err := s.q.StartSession(ctx, queries.StartSessionParams{
		ID:              u.ID,
		Hash:            u.Hash,
		Username:        u.Username,
		ProfileImageURL: u.ProfileImageURL,
		UserID:          u.UserID,
		ExpiresAt:       u.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("session: insert: %w", err)
	}

	s.setCookie(w, u)
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
func (s *Store) Refresh(ctx context.Context, w http.ResponseWriter, u *UserSession) error {
	now := time.Now()
	expiresAt := nextExpiry(now, u.CreatedAt)

	result, err := s.q.RefreshSession(ctx, queries.RefreshSessionParams{
		ExpiresAt:      expiresAt,
		Hash:           u.Hash,
		RefreshCutoff:  now.Add(-refreshInterval),
		LifetimeCutoff: now.Add(-MaxLifetime),
	})
	if err != nil {
		return fmt.Errorf("session: refresh: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: refresh: %w", err)
	}
	if rows == 0 {
		// NOTE: throttled, or the session has hit MaxLifetime and is being
		// allowed to run out its remaining window
		return nil
	}

	u.ExpiresAt = expiresAt
	s.setCookie(w, u)
	return nil
}

// Logout ends the session the request's cookie names and clears the cookie.
// A request with no cookie is already logged out and is not an error.
func (s *Store) Logout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil
		}
		return err
	}

	token, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return fmt.Errorf("session: decode cookie: %w", err)
	}

	if err := s.q.EndSession(r.Context(), hashToken(token)); err != nil {
		return fmt.Errorf("session: end: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Store) setCookie(w http.ResponseWriter, u *UserSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    base64.RawURLEncoding.EncodeToString(u.token),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  u.ExpiresAt,
	})
}

// hashToken is the one-way step between the cookie and the row. SHA-256 is
// enough here: the token is 32 random bytes, so there is nothing to guess.
func hashToken(token []byte) []byte {
	sum := sha256.Sum256(token)
	return sum[:]
}
