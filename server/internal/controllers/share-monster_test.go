package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testImporterID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS7")

func monsterShareGrant() queries.GetShareByTokenRow {
	return queries.GetShareByTokenRow{
		ID:           testAssetID,
		OwnerID:      testOwnerID,
		ResourceType: queries.SharesResourceTypeMonster,
		ResourceID:   testMonsterID,
	}
}
func TestTheImportIsOfferedToAReaderAndNotToTheOwner(t *testing.T) {
	signedOut := sharedMonsterActions(ulid.ULID{}, "tok", testOwnerID)
	if signedOut.SignIn == "" || signedOut.Import != "" {
		t.Errorf("a signed-out reader is offered %+v, want the sign-in link alone", signedOut)
	}
	reader := sharedMonsterActions(testImporterID, "tok", testOwnerID)
	if reader.Import != "/share/tok/import" || reader.SignIn != "" {
		t.Errorf("a signed-in reader is offered %+v, want the import alone", reader)
	}
	owner := sharedMonsterActions(testOwnerID, "tok", testOwnerID)
	if owner.Import != "" || owner.SignIn != "" {
		t.Errorf("the owner is offered %+v, want no import at all", owner)
	}
}
func TestTheActionsRowIsNeverJustAButton(t *testing.T) {
	for name, viewer := range map[string]ulid.ULID{
		"signed out": {},
		"the reader": testImporterID,
		"the owner":  testOwnerID,
	} {
		t.Run(name, func(t *testing.T) {
			actions := sharedMonsterActions(viewer, "tok", testOwnerID)
			if actions.Blurb == "" {
				t.Error("the row has nothing beside its buttons")
			}
			if !strings.Contains(actions.Blurb, "Markdown") {
				t.Errorf("the sentence does not mention the one action every reader gets: %q", actions.Blurb)
			}
		})
	}
}
func TestEveryReaderOfASharedMonsterIsOfferedTheExport(t *testing.T) {
	for name, viewer := range map[string]ulid.ULID{
		"signed out": {},
		"the reader": testImporterID,
		"the owner":  testOwnerID,
	} {
		t.Run(name, func(t *testing.T) {
			if got := sharedMonsterActions(viewer, "tok", testOwnerID).Export; got != "/share/tok/export.md" {
				t.Errorf("export = %q, want the share's own download", got)
			}
		})
	}
}
func TestAnImportWithABadTokenRunsNoStatements(t *testing.T) {
	app, db := newPanelApp(1)
	r := httptest.NewRequest(http.MethodPost, "/share/nope/import", nil)
	r.SetPathValue("token", "nope")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testImporterID}))
	rec := httptest.NewRecorder()
	app.ImportSharedMonster(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 || len(db.reads) != 0 {
		t.Errorf("a malformed token reached the database")
	}
}
func TestAnImportedMonsterIsWrittenUnderTheReaderAndReadOffTheShare(t *testing.T) {
	app, db := newPanelApp(1)
	if err := copyMonster(context.Background(), app.Queries, monsterShareGrant(), testImporterID, ulid.Make()); err == nil {
		t.Fatal("copyMonster succeeded against a database that answers no reads")
	}
	if len(db.calls) == 0 {
		t.Fatal("the import ran no statements at all")
	}
	insert := db.calls[0]
	if !strings.Contains(insert.query, "INSERT INTO monsters") {
		t.Fatalf("the first statement is not the copy:\n%s", insert.query)
	}
	ids := make([]ulid.ULID, 0, len(insert.args))
	for _, arg := range insert.args {
		if id, ok := boundID(arg); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) != 4 {
		t.Fatalf("the copy takes %d ids, want the new monster, the reader, the original and its owner", len(ids))
	}
	for i, want := range []ulid.ULID{testImporterID, testMonsterID, testOwnerID} {
		if ids[i+1] != want {
			t.Errorf("copy id %d = %v, want %v", i+1, ids[i+1], want)
		}
	}
	if ids[0].IsZero() || ids[0] == testMonsterID {
		t.Errorf("the copy is written at %v, want an id of its own", ids[0])
	}
	read := db.calls[1]
	if !strings.Contains(read.query, "FROM monster_actions") {
		t.Fatalf("the second statement does not read the original's rows:\n%s", read.query)
	}
	for i, want := range []ulid.ULID{testMonsterID, testOwnerID} {
		if got, ok := boundID(read.args[i]); !ok || got != want {
			t.Errorf("the rows are read with %v, want %v", read.args[i], want)
		}
	}
}
func TestAMonsterDeletedBeforeTheButtonIsPressedIsADeadLink(t *testing.T) {
	app, db := newPanelApp(0)
	err := copyMonster(context.Background(), app.Queries, monsterShareGrant(), testImporterID, ulid.Make())
	if !errors.Is(err, errImportedMonsterGone) {
		t.Fatalf("copyMonster error = %v, want %v", err, errImportedMonsterGone)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want the copy alone: %v", len(db.calls), deleteTargets(t, db.calls[1:]))
	}
}
func TestAFailedImportWritesNoCompensatingDelete(t *testing.T) {
	app, db := newPanelApp(1)
	monsterID := ulid.Make()
	err := copyMonster(context.Background(), app.Queries, monsterShareGrant(), testImporterID, monsterID)
	if err == nil {
		t.Fatal("copyMonster succeeded against a database that answers no reads")
	}
	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want the copy and the read that failed", len(db.calls))
	}
	for _, call := range append(db.calls, db.reads...) {
		if strings.Contains(strings.ToUpper(call.query), "DELETE") {
			t.Errorf("a failed import ran a delete; the transaction is the rollback:\n%s", call.query)
		}
	}
	for _, call := range db.calls {
		for _, arg := range call.args {
			id, ok := boundID(arg)
			if !ok {
				continue
			}
			if id == testMonsterID && !strings.Contains(call.query, "SELECT") {
				t.Errorf("a write names the original:\n%s", call.query)
			}
		}
	}
}
func TestTxCommitsOnSuccessAndRollsBackOnFailure(t *testing.T) {
	for name, c := range map[string]struct {
		fn        func(*queries.Queries) error
		wantEnd   string
		wantError bool
	}{
		"success": {func(*queries.Queries) error { return nil }, "commit", false},
		"failure": {func(*queries.Queries) error { return errors.New("no") }, "rollback", true},
	} {
		t.Run(name, func(t *testing.T) {
			conn := &recordingConn{}
			app := &App{DB: sql.OpenDB(recordingConnector{conn}), Queries: queries.New(refusingDB)}
			err := app.tx(context.Background(), func(q *queries.Queries) error {
				if q == app.Queries {
					t.Error("fn was handed App's own Queries rather than one bound to the transaction")
				}
				return c.fn(q)
			})
			if c.wantError != (err != nil) {
				t.Errorf("tx error = %v, wantError %v", err, c.wantError)
			}
			if conn.ended != c.wantEnd {
				t.Errorf("the transaction was %sed, want %s", conn.ended, c.wantEnd)
			}
		})
	}
}

type recordingConnector struct{ conn *recordingConn }

func (c recordingConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c recordingConnector) Driver() driver.Driver                        { return nil }

type recordingConn struct{ ended string }

func (c *recordingConn) Prepare(string) (driver.Stmt, error) { return nil, io.ErrUnexpectedEOF }
func (c *recordingConn) Close() error                        { return nil }
func (c *recordingConn) Begin() (driver.Tx, error)           { return c, nil }
func (c *recordingConn) Commit() error {
	c.ended = "commit"
	return nil
}
func (c *recordingConn) Rollback() error {
	c.ended = "rollback"
	return nil
}
func copiedColumns(t *testing.T) map[string]bool {
	t.Helper()
	statement, ok := namedStatements(t, "monsters.sql")["CopyMonster"]
	if !ok {
		t.Fatal("no CopyMonster statement in sql/monsters.sql")
	}
	list := regexp.MustCompile(`(?s)INSERT INTO monsters \((.*?)\)`).FindStringSubmatch(statement)
	if list == nil {
		t.Fatalf("CopyMonster is not an insert into monsters:\n%s", statement)
	}
	columns := map[string]bool{}
	for _, column := range strings.Split(list[1], ",") {
		columns[strings.Trim(strings.TrimSpace(column), "`")] = true
	}
	return columns
}
func TestAnImportedMonsterCarriesEveryEditableColumn(t *testing.T) {
	copied := copiedColumns(t)
	for _, column := range tableColumns(t, "monsters") {
		own := column == "id" || column == "owner_id"
		switch {
		case unownedMonsterColumns[column] && !own:
			if copied[column] {
				t.Errorf("an import copies %q, which does not belong to the copy", column)
			}
		case !copied[column]:
			t.Errorf("an import does not carry %q, so every copy takes the schema's default for it", column)
		}
	}
}
