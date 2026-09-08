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

	// RefreshedAt is when this row's expiry was last slid forward, carried on
	// the request so Refresh can decide whether to write without asking.
	//
	// IT IS HERE TO SAVE A ROUND TRIP AND NOT TO CHANGE A RULE. The throttle
	// has always lived in RefreshSession's WHERE, which is where it has to stay
	// -- two requests from two tabs read the same row before either writes, and
	// only the statement can settle that. What the column bought by coming
	// along with the read is that the common case, which is almost every
	// request, no longer sends an UPDATE across the network to match no rows.
	RefreshedAt time.Time

	// Prefs is the account settings the page renders with: theme, zone, date
	// order and clock.
	//
	// IT IS JOINED, NOT COPIED INTO THE ROW, and so is Username above. Those
	// all change while the user is sitting in the app, and one user has several
	// sessions, so a copy would mean switching to dark on a laptop -- or
	// renaming yourself -- and watching the phone show the old value until its
	// session expired a week later.
	//
	// ProfileImageURL is the one that is still a copy on the sessions row, and
	// that is right for it: it comes from Clerk, this app has no way to change
	// it, and a login is the only thing that ever refreshes it.
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
		RefreshedAt:     row.RefreshedAt,
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
// UserID and ProfileImageURL must be set. The ID, token and timestamps are
// filled in here.
//
// THE NAME IS NOT WRITTEN, because the row does not hold one -- it is read off
// the join to users on every request. u.Username is still set by the caller,
// since the session it is handed back is the one that request renders with.
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
// session past MaxLifetime.
//
// The cookie has to be re-issued alongside the row: leaving it on its original
// Expires would have the browser drop it while the session was still live.
//
// THIS RUNS ON EVERY PAGE, EVERY FRAGMENT AND EVERY POST, which is why the
// throttle is checked twice. The row we were handed already says when it was
// last refreshed, so the throttled case -- which is nearly all of them, one
// write an hour against a request every few seconds -- returns here without a
// statement. The WHERE in RefreshSession stays exactly as it was and is the
// correctness half: it is what settles two tabs reading the same row at the
// same moment, and what enforces MaxLifetime, and neither of those can be
// decided from a copy taken before the write.
func (s *Store) Refresh(ctx context.Context, w http.ResponseWriter, u *UserSession) error {
	now := time.Now()
	if now.Sub(u.RefreshedAt) < refreshInterval {
		return nil
	}

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

// EndCurrent ends whatever session the request's cookie names and leaves the
// cookie alone, for a caller that is about to issue a fresh one.
//
// IT IS WHAT /authorize OWES THE ROW IT IS REPLACING. Signing in inserts a
// session and overwrites the cookie, so the row the browser held a second
// earlier goes on being valid for up to a week idle and a month absolute with
// nothing pointing at it -- and a token copied off that browser would have
// exactly that window to be used in. Ending it first closes the window, and
// the hourly sweep goes back to collecting rows that ran out rather than rows
// nobody ever logged out of.
//
// A request with no cookie, or one that does not decode, is nothing to end and
// is not an error: it is somebody signing in for the first time, or with a
// cookie from a build that wrote them differently.
func (s *Store) EndCurrent(r *http.Request) error {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}

	token, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}

	return s.q.EndSession(r.Context(), hashToken(token))
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

// ErrSessionGone is what the two room writes below report when their statement
// matched nothing. The only way that happens is that the row ended between the
// auth middleware reading it and the handler writing it -- a logout in another
// tab, or a sweep of an expired session. It is an error rather than a silent
// success because the caller is about to tell somebody they joined a room they
// are not in.
var ErrSessionGone = errors.New("session: the session ended before the write landed")

// JoinRoom seats this session at a room, with the character they picked or none
// at all. It is a method on Store rather than a query the handler runs, because
// q is unexported and this package is the only code that touches the row -- the
// same rule that keeps the cookie and the expiry in step.
//
// IT MUTATES u AS WELL AS THE ROW. The handler holds a copy of the session
// taken by the middleware a moment ago, and it is that copy the page renders
// from; leaving it stale would mean joining a room and being told by the very
// next line of the handler that you are not in one. The cookie is untouched,
// because it names the row and the row is what changed -- the middleware reads
// it fresh on every request.
func (s *Store) JoinRoom(ctx context.Context, u *UserSession, roomID ulid.ULID, characterID *ulid.ULID) error {
	result, err := s.q.SetSessionRoom(ctx, queries.SetSessionRoomParams{
		RoomID:      &roomID,
		CharacterID: characterID,
		Hash:        u.Hash,
	})
	if err != nil {
		return fmt.Errorf("session: join room: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: join room: %w", err)
	}
	if rows == 0 {
		return ErrSessionGone
	}

	u.RoomID = &roomID
	u.CharacterID = characterID

	return nil
}

// LeaveRoom takes this session back out of whatever room it was in, and clears
// the character with it: it was chosen for that table and means nothing at the
// next one. It is the mirror of JoinRoom in every respect, including mutating
// the caller's copy.
func (s *Store) LeaveRoom(ctx context.Context, u *UserSession) error {
	result, err := s.q.ClearSessionRoom(ctx, u.Hash)
	if err != nil {
		return fmt.Errorf("session: leave room: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session: leave room: %w", err)
	}
	if rows == 0 {
		return ErrSessionGone
	}

	u.RoomID = nil
	u.CharacterID = nil

	return nil
}
