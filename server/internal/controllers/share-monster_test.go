package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// The import, which is the only write any visitor to a shared page can cause.
// It is bounded the way every other share test is -- recordingDB answers a :one
// by failing, so the handler cannot be driven past GetShareByToken and the page
// itself cannot be rendered here -- so what is covered is the half that holds
// the property: which ids the copy is written from and under, what happens to a
// copy that cannot be finished, and the three states of the offer the page puts
// in front of a reader.

// The reader taking the copy, who is not the owner of the monster being copied.
// Every assertion below is about telling those two apart.
var testImporterID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS7")

func monsterShareGrant() queries.GetShareByTokenRow {
	return queries.GetShareByTokenRow{
		ID:           testAssetID,
		OwnerID:      testOwnerID,
		ResourceType: queries.SharesResourceTypeMonster,
		ResourceID:   testMonsterID,
	}
}

// THE THREE STATES ARE THE THREE KINDS OF READER, and the owner's is the one
// worth pinning: importing your own monster would hand you a duplicate you did
// not ask for, and a button that did it would be indistinguishable from a button
// that had failed and reloaded the page.
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
	if owner.Import != "" || owner.SignIn != "" || owner.Blurb != "" {
		t.Errorf("the owner is offered %+v, want no import at all", owner)
	}
}

// THE EXPORT IS THE ONE ACTION EVERY READER GETS, and that is deliberate rather
// than an oversight in the switch above: the file holds exactly what the page
// holds, so there is nobody who can read this monster and should not be able to
// keep the text of it. The import is the one that writes, and only that one is
// conditional.
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

// A token that is not shaped like one of ours cannot name a row, so it is
// refused before anything is queried -- the same cheap refusal every other
// share route opens with. The answer is the dead-link page rather than a
// redirect, because there is nobody signed in to send anywhere.
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

// THE COPY IS WRITTEN UNDER THE READER AND READ OFF THE SHARE ROW, which is the
// whole of what stops an import being a way to reach a monster the link does not
// name. The four ids in this statement are the only ones it has: two minted or
// taken from the session, two off the grant, and nothing from the request.
func TestAnImportedMonsterIsWrittenUnderTheReaderAndReadOffTheShare(t *testing.T) {
	app, db := newPanelApp(1)

	// The action copy cannot succeed against this harness -- the read that
	// feeds it comes back empty-handed -- so the failure is expected, and the
	// statement that ran before it is what this is about. The new id is taken
	// off that statement rather than off the return value, which a failed
	// import deliberately leaves empty.
	if _, err := app.importMonster(context.Background(), monsterShareGrant(), testImporterID); err == nil {
		t.Fatal("importMonster succeeded against a database that answers no reads")
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

	// The rows it copies are read through the grant as well. Reading them under
	// the importer would find nothing and produce a monster with an empty stat
	// block rather than a failure.
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

// Zero rows inserted is the monster having been deleted between the page
// rendering and the button being pressed -- the id was freshly minted, so a
// duplicate key is not on the table. It is the dead link every other miss is,
// and nothing has been written for a rollback to undo.
func TestAMonsterDeletedBeforeTheButtonIsPressedIsADeadLink(t *testing.T) {
	app, db := newPanelApp(0)

	_, err := app.importMonster(context.Background(), monsterShareGrant(), testImporterID)
	if !errors.Is(err, errImportedMonsterGone) {
		t.Fatalf("importMonster error = %v, want %v", err, errImportedMonsterGone)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want the copy alone: %v", len(db.calls), deleteTargets(t, db.calls[1:]))
	}
}

// A HALF-COPIED MONSTER IS WORSE THAN NO COPY AT ALL, because nothing about it
// says so: it sits in the reader's manual with its name, its armour class and
// none of its actions, looking exactly like a monster whose owner never wrote
// any. Nothing in this schema is transactional, so the rollback is written out
// -- and it runs the monster delete's own statements, so a table added to that
// purge is covered here too.
func TestAnImportThatCannotBeFinishedIsThrownAway(t *testing.T) {
	app, db := newPanelApp(1)

	monsterID, err := app.importMonster(context.Background(), monsterShareGrant(), testImporterID)
	if err == nil {
		t.Fatal("importMonster succeeded against a database that answers no reads")
	}
	if !monsterID.IsZero() {
		t.Errorf("a failed import handed back %v, want nothing to redirect to", monsterID)
	}

	// The copy and the read that failed, then the purge.
	if len(db.calls) < 3 {
		t.Fatalf("ran %d statements, want the copy, the read and a rollback", len(db.calls))
	}
	emptied := deleteTargets(t, db.calls[2:])
	if !strings.Contains(strings.Join(emptied, ","), "monsters") {
		t.Errorf("the half-made monster was left behind; the rollback emptied %v", emptied)
	}

	// EVERY ROLLBACK STATEMENT NAMES THE READER AND THE NEW ROW. One that named
	// the original instead would delete the monster this import was copying --
	// out of somebody else's manual, on their own share link.
	for _, call := range db.calls[2:] {
		for _, arg := range call.args {
			id, ok := boundID(arg)
			if !ok {
				continue
			}
			if id == testOwnerID || id == testMonsterID {
				t.Errorf("the rollback reaches the original:\n%s", call.query)
			}
		}
	}
}

// copiedColumns is the column list the copy writes, taken off the statement
// rather than off the Go, because the statement is where a column goes missing.
// Only the INSERT's own list is read: the prose above it names columns too, and
// a comment mentioning asset_id is not the statement writing one.
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

// A COLUMN LEFT OUT OF THE COPY DOES NOT FAIL, IT LIES. Every column in this
// table carries a DEFAULT -- that is what lets a monster be created from a name
// alone -- so a copy that forgot `ac` would hand the reader a dragon with an
// armour class of 10 and nothing to say it had happened.
//
// The three the copy must NOT carry are the ones that are not the monster:
// created_at and updated_at belong to the database, and asset_id names an object
// under the original owner's prefix that this account cannot keep -- the picture
// is copied as a new object instead.
func TestAnImportedMonsterCarriesEveryEditableColumn(t *testing.T) {
	copied := copiedColumns(t)

	for _, column := range tableColumns(t, "monsters") {
		// The copy writes its own identity, which the panels do not: it is a
		// new row in a different account, so the two ids no panel may touch are
		// exactly the two this statement has to.
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
