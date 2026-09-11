package middleware

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
)

type sessionConnector struct{}

func (sessionConnector) Connect(context.Context) (driver.Conn, error) { return sessionConn{}, nil }
func (sessionConnector) Driver() driver.Driver                        { return nil }

type sessionConn struct{}

func (c sessionConn) Prepare(query string) (driver.Stmt, error) { return sessionStmt{query}, nil }
func (c sessionConn) Close() error                              { return nil }
func (c sessionConn) Begin() (driver.Tx, error)                 { return nil, io.ErrUnexpectedEOF }

type sessionStmt struct{ query string }

func (s sessionStmt) Close() error                               { return nil }
func (s sessionStmt) NumInput() int                              { return -1 }
func (s sessionStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (s sessionStmt) Query([]driver.Value) (driver.Rows, error) {
	if !strings.Contains(s.query, "FROM sessions") {
		return nil, io.ErrUnexpectedEOF
	}
	return &sessionRows{}, nil
}

type sessionRows struct{ done bool }

func (r *sessionRows) Columns() []string {
	return []string{
		"id", "profile_image_url", "user_id", "character_id", "room_id",
		"created_at", "refreshed_at",
		"username", "avatar_asset_id",
		"theme", "timezone", "date_format", "time_format",
		"follow_turn", "show_blood", "ping_volume", "onboarded_at",
	}
}
func (r *sessionRows) Close() error { return nil }
func (r *sessionRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	now := time.Now()
	for i, v := range []driver.Value{
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",
		[]byte(""),
		"01BX5ZZKBKACTAV9WEVGEMMVRZ",
		nil,
		nil,
		now.Add(-time.Hour),
		now,
		[]byte("gm"),
		nil,
		[]byte("system"),
		[]byte("UTC"),
		[]byte("iso"),
		[]byte("24h"),
		int64(1),
		int64(1),
		int64(100),
		nil,
	} {
		dest[i] = v
	}
	return nil
}

var signedIn = Auth{Sessions: session.NewStore(queries.New(sql.OpenDB(sessionConnector{})), false)}

const sessionCookie = "session_id=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
