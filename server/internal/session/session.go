package session

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/queries"
	"net/http"

	"github.com/oklog/ulid/v2"
)

type UserSession struct {
	Id              ulid.ULID
	UserId          ulid.ULID
	CharacterId     ulid.ULID
	RoomId          ulid.ULID
	Username        string
	ProfileImageURL string
}

func GetUserSessionFromCookie(r *http.Request, db *sql.DB) (UserSession, error) {
	session := UserSession{}
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if err == http.ErrNoCookie {
			return session, err
		}
		slog.Error("Failed to get user session from cookie", "error", err)
		return session, err
	}

	ctx := context.Background()
	q := queries.New(db)
	result, err := q.GetSession(ctx, []byte(cookie.Value))
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
