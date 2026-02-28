package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"
	"main/internal/queries"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type UserSession struct {
	Id              ulid.ULID
	UserId          ulid.ULID
	CharacterId     ulid.ULID
	RoomId          ulid.ULID
	Username        string
	ProfileImageURL string
	Hash            []byte
	ExpiresAt       time.Time
}

func New() *UserSession {
	s := UserSession{}
	s.Id = ulid.Make()
	return &s
}

func GetUserSessionFromCookie(r *http.Request, db *sql.DB, ctx context.Context) (UserSession, error) {
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
	result, err := q.GetSession(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session, nil
		}
		slog.Error("Failed to get user session from DB", "error", err)
		return session, err
	}

	session.Id = ulid.ULID(result.ID)
	session.UserId = ulid.ULID(result.UserID)
	session.Username = result.Username
	session.ProfileImageURL = result.ProfileImageUrl
	if result.CharacterID.Valid {
		b := []byte(result.CharacterID.String)
		session.CharacterId = ulid.ULID(b)
	}
	if result.RoomID.Valid {
		b := []byte(result.RoomID.String)
		session.RoomId = ulid.ULID(b)
	}

	return session, nil
}

func (s *UserSession) CreateSession(db *sql.DB, ctx context.Context) error {
	q := queries.New(db)
	s.Hash = make([]byte, 32)
	if _, err := rand.Read(s.Hash); err != nil {
		return err
	}
	//s.ExpiresAt = time.Now().Add(3 * 24 * time.Hour)
	s.ExpiresAt = time.Now().Add(1 * time.Hour)
	err := q.StartSession(ctx, queries.StartSessionParams{
		ID:              s.Id[:],
		Username:        s.Username,
		ProfileImageUrl: s.ProfileImageURL,
		UserID:          s.UserId[:],
		ExpiresAt:       s.ExpiresAt,
		Hash:            s.Hash,
	})
	if err != nil {
		return err
	}
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

func (s UserSession) SetCookie(w http.ResponseWriter) {
	hash := s.EncodeHash()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    hash,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.ExpiresAt,
	})
}

func Logout(r *http.Request, w http.ResponseWriter, db *sql.DB, ctx context.Context) error {
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
	err = q.EndSession(ctx, hash)
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
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}
