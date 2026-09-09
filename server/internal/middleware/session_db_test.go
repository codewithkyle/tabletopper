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

// A database that answers GetSession with one live session, so the wrappers in
// this package can be tested on the path where somebody IS signed in.
//
// IT IS A driver AND NOT A DBTX, and that is not a preference. GetSession scans
// a *sql.Row, and a *sql.Row that scans successfully cannot be constructed
// outside database/sql -- so a stub at the queries.DBTX seam can only ever
// return the failure the controller tests use it for. One layer lower, a
// driver.Connector handing back a single row is enough, and it is the only
// place a signed-in request can be faked from.
//
// THE COLUMN LIST BELOW MIRRORS sql/session.sql. If GetSession gains a column
// this fails with a scan error naming the count, which is the right kind of
// loud: add the column here. Nothing else in the app reads this stub.
type sessionConnector struct{}

func (sessionConnector) Connect(context.Context) (driver.Conn, error) { return sessionConn{}, nil }

func (sessionConnector) Driver() driver.Driver { return nil }

type sessionConn struct{}

func (c sessionConn) Prepare(query string) (driver.Stmt, error) { return sessionStmt{query}, nil }

func (c sessionConn) Close() error { return nil }

func (c sessionConn) Begin() (driver.Tx, error) { return nil, io.ErrUnexpectedEOF }

type sessionStmt struct{ query string }

func (s sessionStmt) Close() error  { return nil }
func (s sessionStmt) NumInput() int { return -1 }

// The refresh UPDATE, which matched nothing: the row the stub hands back was
// refreshed a moment ago, so Refresh returns before it gets here. Answering
// rather than failing keeps this stub honest if that ever changes.
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
		"theme", "timezone", "date_format", "time_format", "follow_turn", "onboarded_at",
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
		// The two ULIDs are strings and not []byte: ULID.Scan reads a string as
		// the 26-character text form and a []byte as 16 raw bytes, and it is
		// the text form that a MySQL CHAR(26) hands back.
		"01ARZ3NDEKTSV4RRFFQ69G5FAV", // id
		[]byte(""),                   // profile_image_url
		"01BX5ZZKBKACTAV9WEVGEMMVRZ", // user_id
		nil,                          // character_id
		nil,                          // room_id
		now.Add(-time.Hour),          // created_at
		now,                          // refreshed_at, so Refresh skips its UPDATE
		[]byte("gm"),                 // username, off the join to users
		nil,                          // avatar_asset_id: no uploaded picture
		[]byte("system"),             // theme
		[]byte("UTC"),                // timezone
		[]byte("iso"),                // date_format
		[]byte("24h"),                // time_format
		int64(1),                     // follow_turn
		nil,                          // onboarded_at
	} {
		dest[i] = v
	}

	return nil
}

// signedIn is an Auth whose store answers with the session above, and the
// cookie a request has to carry to get it. The token's bytes are never checked
// against anything -- the stub answers any hash -- so any 32 base64url
// characters will do.
var signedIn = Auth{Sessions: session.NewStore(queries.New(sql.OpenDB(sessionConnector{})), false)}

const sessionCookie = "session_id=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
