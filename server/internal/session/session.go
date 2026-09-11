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
	IdleWindow      = 7 * 24 * time.Hour
	MaxLifetime     = 30 * 24 * time.Hour
	refreshInterval = time.Hour
	cookieName      = "session_id"
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
	RefreshedAt     time.Time
	Prefs           prefs.Preferences
	Onboarded       bool
	token           []byte
}
type Store struct {
	q      *queries.Queries
	secure bool
}

func NewStore(q *queries.Queries, secure bool) *Store {
	return &Store{q: q, secure: secure}
}
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
	return UserSession{
		ID:              row.ID,
		UserID:          row.UserID,
		CharacterID:     row.CharacterID,
		RoomID:          row.RoomID,
		Username:        row.Username,
		ProfileImageURL: AvatarURL(row.AvatarAssetID, row.ProfileImageURL),
		Hash:            hash,
		CreatedAt:       row.CreatedAt,
		RefreshedAt:     row.RefreshedAt,
		Prefs: prefs.New(
			string(row.Theme),
			row.Timezone,
			string(row.DateFormat),
			string(row.TimeFormat),
			row.FollowTurn,
			row.ShowBlood,
			int(row.PingVolume),
		),
		Onboarded: row.OnboardedAt.Valid,
		token:     token,
	}, nil
}
func AvatarURL(uploaded *ulid.ULID, clerk string) string {
	if uploaded != nil {
		return "/assets/images/" + uploaded.String()
	}
	return clerk
}
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
func nextExpiry(now time.Time, createdAt time.Time) time.Time {
	expiresAt := now.Add(IdleWindow)
	if cap := createdAt.Add(MaxLifetime); expiresAt.After(cap) {
		return cap
	}
	return expiresAt
}
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
		return nil
	}
	u.ExpiresAt = expiresAt
	s.setCookie(w, u)
	return nil
}
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
func hashToken(token []byte) []byte {
	sum := sha256.Sum256(token)
	return sum[:]
}

var ErrSessionGone = errors.New("session: the session ended before the write landed")

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
