package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

// roomDB is the third database stub in this package, and it exists because the
// room handlers ask two things neither of the other two can answer.
//
// recordingDB is a queries.DBTX: it reports the statement a handler wrote, and
// it cannot hand back a row, because a *sql.Rows can only be built by
// database/sql. oneRowDB is a driver.Connector and can, but it answers every
// query with the same row and refuses to begin a transaction.
//
// The room handlers need both at once. The page reads the room and then its
// members -- two queries with different shapes -- and the close and the delete
// are each two statements inside one a.tx, which needs a real *sql.DB that can
// Begin. So this records like the first, answers like the second with a
// different row per query in the order they are asked, and commits.
//
// NOTHING HERE DISPATCHES ON SQL. Answers are consumed in order, so a test
// using this says what its handler reads and in which order -- and a handler
// that grew a third read would run off the end and see zero rows, which is a
// failure the test would see rather than a stub quietly obliging.
type roomDB struct {
	// answers are the result sets the queries get, in order. A query past the
	// end of the list sees no rows at all.
	answers []roomAnswer

	// rows is what every Exec reports as matched. ClientFoundRows is on in the
	// real pool, so this is "matched" and not "changed": 1 is a statement that
	// found its row and 0 is one that did not.
	rows int64

	calls []recordedCall
	next  int
}

// roomAnswer is one result set: the columns and the single row, or no row at
// all when values is nil.
type roomAnswer struct {
	columns []string
	values  []driver.Value
}

func (d *roomDB) db() *sql.DB { return sql.OpenDB(roomConnector{d}) }

// only is recordingDB.only for this stub: the one statement the handler ran.
func (d *roomDB) only(t *testing.T) recordedCall {
	t.Helper()

	if len(d.calls) != 1 {
		t.Fatalf("statements run = %d, want 1: %v", len(d.calls), d.queries())
	}

	return d.calls[0]
}

// queries is the statements in the order they went out, trimmed to their first
// line so a failure message names them rather than printing the whole SQL.
func (d *roomDB) queries() []string {
	sent := make([]string, 0, len(d.calls))
	for _, call := range d.calls {
		sent = append(sent, strings.TrimSpace(strings.SplitN(strings.TrimSpace(call.query), "\n", 2)[0]))
	}

	return sent
}

type roomConnector struct{ stub *roomDB }

func (c roomConnector) Connect(context.Context) (driver.Conn, error) { return roomConn{c.stub}, nil }
func (c roomConnector) Driver() driver.Driver                        { return nil }

type roomConn struct{ stub *roomDB }

func (c roomConn) Prepare(query string) (driver.Stmt, error) {
	return roomStmt{stub: c.stub, query: query}, nil
}
func (c roomConn) Close() error { return nil }

// Begin is what separates this from oneRowDB. a.tx needs it, and the
// transaction itself is a no-op: what the tests assert is which statements went
// out and in what order, and a rollback that undid nothing would be
// indistinguishable from a commit either way. The handlers under test decide
// between the two by returning an error, which is asserted on the response.
func (c roomConn) Begin() (driver.Tx, error) { return roomTx{}, nil }

type roomTx struct{}

func (roomTx) Commit() error   { return nil }
func (roomTx) Rollback() error { return nil }

type roomStmt struct {
	stub  *roomDB
	query string
}

func (s roomStmt) Close() error  { return nil }
func (s roomStmt) NumInput() int { return -1 }

func (s roomStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.stub.record(s.query, args)

	return driver.RowsAffected(s.stub.rows), nil
}

func (s roomStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.stub.record(s.query, args)

	answer := roomAnswer{}
	if s.stub.next < len(s.stub.answers) {
		answer = s.stub.answers[s.stub.next]
	}
	s.stub.next++

	return &roomRows{answer: answer, done: answer.values == nil}, nil
}

// record keeps the statement and its values. The values are driver.Value by the
// time they arrive here, so a ULID is sixteen bytes rather than a ulid.ULID --
// see boundRoomID, which is what the assertions compare against.
func (d *roomDB) record(query string, args []driver.Value) {
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, arg)
	}

	d.calls = append(d.calls, recordedCall{query: query, args: values})
}

type roomRows struct {
	answer roomAnswer
	done   bool
}

func (r *roomRows) Columns() []string { return r.answer.columns }
func (r *roomRows) Close() error      { return nil }

func (r *roomRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true

	copy(dest, r.answer.values)

	return nil
}

// boundRoomID reads a ULID out of an argument recorded at the driver level. Both
// ulid.ULID and *ulid.ULID marshal to their sixteen bytes on the way through
// database/sql, so there is one form to read here where recordingDB has two.
func boundRoomID(arg any) (ulid.ULID, bool) {
	raw, ok := arg.([]byte)
	if !ok || len(raw) != 16 {
		return ulid.ULID{}, false
	}

	var id ulid.ULID
	if err := id.UnmarshalBinary(raw); err != nil {
		return ulid.ULID{}, false
	}

	return id, true
}

// getRoomAnswer is the result set GetRoom reads: the seven columns it selects,
// in the order it selects them.
func getRoomAnswer(id ulid.ULID, ownerID ulid.ULID, name string, code string, locked bool, closed bool) roomAnswer {
	var codeValue driver.Value
	if code != "" {
		codeValue = code
	}

	var closedValue driver.Value
	if closed {
		closedValue = time.Now()
	}

	return roomAnswer{
		columns: []string{"id", "owner_id", "name", "code", "is_locked", "created_at", "closed_at"},
		values:  []driver.Value{id.Bytes(), ownerID.Bytes(), name, codeValue, locked, time.Now(), closedValue},
	}
}
