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

type roomDB struct {
	answers []roomAnswer
	rows    int64
	calls   []recordedCall
	next    int
}
type roomAnswer struct {
	columns []string
	values  []driver.Value
}

func (d *roomDB) db() *sql.DB { return sql.OpenDB(roomConnector{d}) }
func (d *roomDB) only(t *testing.T) recordedCall {
	t.Helper()
	if len(d.calls) != 1 {
		t.Fatalf("statements run = %d, want 1: %v", len(d.calls), d.queries())
	}
	return d.calls[0]
}
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
func (c roomConn) Close() error              { return nil }
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
