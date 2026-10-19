package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

type roomDB struct {
	mu      sync.Mutex
	answers []roomAnswer
	named   map[string]roomAnswer
	rows    int64
	calls   []recordedCall
	steps   []string
	next    int
}
type roomAnswer struct {
	columns []string
	values  []driver.Value
	more    [][]driver.Value
}

func (d *roomDB) db() *sql.DB { return sql.OpenDB(roomConnector{d}) }
func (d *roomDB) only(t *testing.T) recordedCall {
	t.Helper()
	calls := d.recorded()
	if len(calls) != 1 {
		t.Fatalf("statements run = %d, want 1: %v", len(calls), d.queries())
	}
	return calls[0]
}
func (d *roomDB) queries() []string {
	calls := d.recorded()
	sent := make([]string, 0, len(calls))
	for _, call := range calls {
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
func (c roomConn) Begin() (driver.Tx, error) {
	c.stub.step("BEGIN")
	return roomTx{c.stub}, nil
}

type roomTx struct{ stub *roomDB }

func (t roomTx) Commit() error {
	t.stub.step("COMMIT")
	return nil
}
func (t roomTx) Rollback() error {
	t.stub.step("ROLLBACK")
	return nil
}

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
	s.stub.mu.Lock()
	defer s.stub.mu.Unlock()
	for name, answer := range s.stub.named {
		if strings.Contains(s.query, "-- name: "+name+" ") {
			return answer.rows(), nil
		}
	}
	answer := roomAnswer{}
	if s.stub.next < len(s.stub.answers) {
		answer = s.stub.answers[s.stub.next]
	}
	s.stub.next++
	return answer.rows(), nil
}
func (a roomAnswer) rows() *roomRows {
	var queue [][]driver.Value
	if a.values != nil {
		queue = append([][]driver.Value{a.values}, a.more...)
	}
	return &roomRows{columns: a.columns, queue: queue}
}
func (d *roomDB) record(query string, args []driver.Value) {
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, arg)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, recordedCall{query: query, args: values})
	d.steps = append(d.steps, query)
}
func (d *roomDB) step(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.steps = append(d.steps, name)
}
func (d *roomDB) recordedSteps() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.steps...)
}
func (d *roomDB) recorded() []recordedCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]recordedCall(nil), d.calls...)
}

type roomRows struct {
	columns []string
	queue   [][]driver.Value
}

func (r *roomRows) Columns() []string { return r.columns }
func (r *roomRows) Close() error      { return nil }
func (r *roomRows) Next(dest []driver.Value) error {
	if len(r.queue) == 0 {
		return io.EOF
	}
	copy(dest, r.queue[0])
	r.queue = r.queue[1:]
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
		columns: []string{"id", "owner_id", "name", "code", "is_locked", "scene_id", "created_at", "closed_at"},
		values:  []driver.Value{id.Bytes(), ownerID.Bytes(), name, codeValue, locked, nil, time.Now(), closedValue},
	}
}
