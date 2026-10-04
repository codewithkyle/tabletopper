package sweep

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

var (
	testUserID  = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ")
	testRoomID  = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT0")
	testCharID  = ulid.MustParse("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	testAssetID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVT5")
)

type statement struct {
	query string
	args  []driver.Value
}
type scriptedDB struct {
	mu    sync.Mutex
	calls []statement
	rows  map[string][]ulid.ULID
	fail  string
}

func (d *scriptedDB) db() *sql.DB { return sql.OpenDB(scriptedConnector{d}) }
func (d *scriptedDB) record(query string, args []driver.Value) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, statement{query: query, args: append([]driver.Value(nil), args...)})
}
func (d *scriptedDB) answer(query string) ([]ulid.ULID, bool) {
	for name, ids := range d.rows {
		if strings.Contains(query, name) {
			return ids, true
		}
	}
	return nil, false
}

type scriptedConnector struct{ stub *scriptedDB }

func (c scriptedConnector) Connect(context.Context) (driver.Conn, error) {
	return scriptedConn{c.stub}, nil
}
func (c scriptedConnector) Driver() driver.Driver { return nil }

type scriptedConn struct{ stub *scriptedDB }

func (c scriptedConn) Prepare(query string) (driver.Stmt, error) {
	return scriptedStmt{stub: c.stub, query: query}, nil
}
func (c scriptedConn) Close() error              { return nil }
func (c scriptedConn) Begin() (driver.Tx, error) { return nil, io.ErrUnexpectedEOF }

type scriptedStmt struct {
	stub  *scriptedDB
	query string
}

func (s scriptedStmt) Close() error  { return nil }
func (s scriptedStmt) NumInput() int { return -1 }
func (s scriptedStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.stub.record(s.query, args)
	if s.stub.fail != "" && strings.Contains(s.query, s.stub.fail) {
		return nil, errors.New("the database refused")
	}
	return driver.RowsAffected(1), nil
}
func (s scriptedStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.stub.record(s.query, args)
	ids, _ := s.stub.answer(s.query)
	return &idRows{ids: ids}, nil
}

type idRows struct {
	ids []ulid.ULID
	at  int
}

func (r *idRows) Columns() []string { return []string{"id"} }
func (r *idRows) Close() error      { return nil }
func (r *idRows) Next(dest []driver.Value) error {
	if r.at >= len(r.ids) {
		return io.EOF
	}
	dest[0] = r.ids[r.at].Bytes()
	r.at++
	return nil
}

type fakeTables struct {
	closed     []ulid.ULID
	characters []ulid.ULID
	assets     []ulid.ULID
}

func (f *fakeTables) Close(_ context.Context, id ulid.ULID) { f.closed = append(f.closed, id) }
func (f *fakeTables) ForgetCharacter(_ context.Context, id ulid.ULID) {
	f.characters = append(f.characters, id)
}
func (f *fakeTables) ForgetAsset(_ context.Context, id ulid.ULID) { f.assets = append(f.assets, id) }

type fakeObjects struct {
	prefixes []string
	err      error
}

func (f *fakeObjects) DeletePrefix(_ context.Context, prefix string) error {
	f.prefixes = append(f.prefixes, prefix)
	return f.err
}
func stockedDB() *scriptedDB {
	return &scriptedDB{rows: map[string][]ulid.ULID{
		"FROM rooms":      {testRoomID},
		"FROM characters": {testCharID},
		"FROM assets":     {testAssetID},
	}}
}
func purge(t *testing.T, db *scriptedDB, objects *fakeObjects) (*fakeTables, error) {
	t.Helper()
	tables := &fakeTables{}
	err := PurgeAccount(context.Background(), queries.New(db.db()), objects, tables, testUserID)
	return tables, err
}
func tablesHoldingUserRows(t *testing.T) []string {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}
	tables := []string{}
	definition := regexp.MustCompile("(?s)CREATE TABLE `([a-z_]+)` \\((.*?)\n\\) ENGINE=")
	for _, match := range definition.FindAllStringSubmatch(string(schema), -1) {
		if strings.Contains(match[2], "`owner_id`") || strings.Contains(match[2], "`user_id`") {
			tables = append(tables, match[1])
		}
	}
	if len(tables) == 0 {
		t.Fatal("no table in db/schema.sql carries an owner_id or a user_id")
	}
	return tables
}
func deleteTargets(calls []statement) []string {
	target := regexp.MustCompile("(?i)DELETE\\s+FROM\\s+`?([a-z_]+)`?")
	targets := []string{}
	for _, call := range calls {
		if match := target.FindStringSubmatch(call.query); match != nil {
			targets = append(targets, match[1])
		}
	}
	return targets
}
func boundUser(call statement) bool {
	for _, arg := range call.args {
		if raw, ok := arg.([]byte); ok && len(raw) == 16 {
			var id ulid.ULID
			if id.UnmarshalBinary(raw) == nil && id == testUserID {
				return true
			}
		}
	}
	return false
}
func TestAPurgeEmptiesEveryTableThatHoldsTheUsersRowsAndTheUserLast(t *testing.T) {
	db := stockedDB()
	if _, err := purge(t, db, &fakeObjects{}); err != nil {
		t.Fatalf("purge: %v", err)
	}
	targets := deleteTargets(db.calls)
	emptied := map[string]bool{}
	for _, table := range targets {
		if emptied[table] {
			t.Errorf("%s is emptied twice", table)
		}
		emptied[table] = true
	}
	for _, table := range append(tablesHoldingUserRows(t), "users") {
		if !emptied[table] {
			t.Errorf("the purge leaves %s behind", table)
		}
		delete(emptied, table)
	}
	for table := range emptied {
		t.Errorf("the purge empties %s, which holds no rows of the user's", table)
	}
	if targets[len(targets)-1] != "users" {
		t.Errorf("the tables were emptied in the order %v; the user row has to go last so a crash leaves the lock in place", targets)
	}
	for i, call := range db.calls {
		if !boundUser(call) {
			t.Errorf("statement %d is not bound to the user:\n%s %v", i, call.query, call.args)
		}
	}
}
func TestAPurgeClosesTheRoomsAndTellsTheHubBeforeTheRowsGo(t *testing.T) {
	db := stockedDB()
	objects := &fakeObjects{}
	tables, err := purge(t, db, objects)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if len(tables.closed) != 1 || tables.closed[0] != testRoomID {
		t.Errorf("rooms closed = %v, want the owned room", tables.closed)
	}
	if len(tables.characters) != 1 || tables.characters[0] != testCharID {
		t.Errorf("characters forgotten = %v, want the owned character", tables.characters)
	}
	if len(tables.assets) != 1 || tables.assets[0] != testAssetID {
		t.Errorf("assets forgotten = %v, want the owned asset", tables.assets)
	}
	if len(objects.prefixes) != 1 || objects.prefixes[0] != "users/"+testUserID.String()+"/" {
		t.Errorf("objects deleted under %v, want the user's whole prefix", objects.prefixes)
	}
	seats, rooms := -1, -1
	for i, call := range db.calls {
		switch {
		case strings.Contains(call.query, "UPDATE sessions") && strings.Contains(call.query, "SELECT id FROM rooms"):
			seats = i
		case strings.Contains(call.query, "DELETE FROM rooms"):
			rooms = i
		}
	}
	if seats < 0 || rooms < 0 || seats > rooms {
		t.Errorf("the players' sessions are cleared at statement %d and the rooms deleted at %d; the rooms must still exist to find the sessions", seats, rooms)
	}
}
func TestAPurgeThatFailsStopsShortOfTheUserRow(t *testing.T) {
	db := stockedDB()
	db.fail = "DELETE FROM monsters"
	_, err := purge(t, db, &fakeObjects{})
	if err == nil {
		t.Fatal("a refused delete did not fail the purge")
	}
	if !strings.Contains(err.Error(), "monsters") {
		t.Errorf("the error does not say which step failed: %v", err)
	}
	for _, table := range deleteTargets(db.calls) {
		if table == "users" {
			t.Fatal("the user row was deleted after an earlier step failed, so the leftovers can never be swept")
		}
	}
}
func TestAPurgeKeepsTheAssetRowsWhenTheObjectsWillNotGo(t *testing.T) {
	db := stockedDB()
	_, err := purge(t, db, &fakeObjects{err: errors.New("the bucket is down")})
	if err == nil {
		t.Fatal("a failed object delete did not fail the purge")
	}
	for _, table := range deleteTargets(db.calls) {
		if table == "assets" || table == "users" {
			t.Fatalf("%s rows were deleted while their objects are still in the bucket", table)
		}
	}
}
