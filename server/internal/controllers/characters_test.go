package controllers

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

// THE PURGE TESTS.
//
// Deleting a character is the one operation in this app that has to name every
// table by hand. There are no foreign keys in this schema, so nothing cascades,
// and a table left out of deleteCharacterRows does not fail -- it leaks. The
// rows stay behind forever unreachable, and the ones in assets keep an object
// in R2 alive with them.
//
// DeleteCharacter itself cannot be driven from here: it opens with a :one that
// recordingDB answers by panicking, and it reaches R2. deleteCharacterRows is
// the part that holds the list, and it is all statements, so it runs.

// tablesHoldingCharacterRows reads db/schema.sql for every table carrying a
// character_id column. It is what makes the test below fail when a table is
// added rather than when someone notices the disk bill.
func tablesHoldingCharacterRows(t *testing.T) []string {
	t.Helper()

	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}

	tables := []string{}
	definition := regexp.MustCompile("(?s)CREATE TABLE `([a-z_]+)` \\((.*?)\n\\) ENGINE=")
	for _, match := range definition.FindAllStringSubmatch(string(schema), -1) {
		if strings.Contains(match[2], "`character_id`") {
			tables = append(tables, match[1])
		}
	}
	if len(tables) == 0 {
		t.Fatal("no table in db/schema.sql carries a character_id")
	}

	return tables
}

// Tables that carry a character_id and are deliberately left alone, with the
// reason, because "we forgot" and "we decided" look identical in a diff.
var unpurgedTables = map[string]string{
	// Nothing writes sessions.character_id. GetSession is the only statement
	// that names the column at all and it reads it; StartSession does not set
	// it and no UPDATE touches it, so the column is NULL in every row and there
	// is nothing for a delete to clear. Purging it would be a statement
	// standing guard over a value that cannot exist.
	"sessions": "never written",
}

// A character's pictures are in assets, which carries no character_id -- it is
// reached through journals.id = assets.journal_id. The schema scan cannot see
// that edge, so the table is named here and the test requires it outright.
const journalImageTable = "assets"

// deleteTargets pulls the table out of each DELETE the calls ran, in order. The
// journal image delete also names journals inside its subquery; that is a
// SELECT, so anchoring on DELETE FROM picks up only what each statement empties.
func deleteTargets(t *testing.T, calls []recordedCall) []string {
	t.Helper()

	target := regexp.MustCompile("(?i)DELETE\\s+FROM\\s+`?([a-z_]+)`?")
	targets := []string{}
	for _, call := range calls {
		match := target.FindStringSubmatch(call.query)
		if match == nil {
			t.Fatalf("not a DELETE:\n%s", call.query)
		}
		targets = append(targets, match[1])
	}

	return targets
}

func TestDeletingACharacterEmptiesEveryTableThatHoldsItsRows(t *testing.T) {
	app, db := newPanelApp(1)

	if err := app.deleteCharacterRows(context.Background(), testCharacterID, testOwnerID); err != nil {
		t.Fatalf("deleteCharacterRows: %v", err)
	}

	emptied := map[string]bool{}
	for _, table := range deleteTargets(t, db.calls) {
		if emptied[table] {
			t.Errorf("%s is emptied twice", table)
		}
		emptied[table] = true
	}

	want := []string{journalImageTable}
	for _, table := range tablesHoldingCharacterRows(t) {
		if reason, skipped := unpurgedTables[table]; skipped {
			t.Logf("%s is not purged: %s", table, reason)
			continue
		}
		want = append(want, table)
	}

	for _, table := range want {
		if !emptied[table] {
			t.Errorf("a character delete leaves %s behind", table)
		}
		delete(emptied, table)
	}
	for table := range emptied {
		t.Errorf("a character delete empties %s, which holds no rows of its own", table)
	}
}

// THE IMAGE ROWS GO BEFORE THE JOURNALS THEY HANG OFF, and the order is the
// test. The delete finds them through journal_id, so after the journals rows
// are gone it would match nothing: every picture in the journal would stay in
// assets forever, with its object in the bucket and no page that could ever
// render it.
func TestJournalImageRowsAreDeletedBeforeTheirJournals(t *testing.T) {
	app, db := newPanelApp(1)

	if err := app.deleteCharacterRows(context.Background(), testCharacterID, testOwnerID); err != nil {
		t.Fatalf("deleteCharacterRows: %v", err)
	}

	targets := deleteTargets(t, db.calls)
	images, journals := -1, -1
	for i, table := range targets {
		switch table {
		case journalImageTable:
			images = i
		case "journals":
			journals = i
		}
	}
	if images < 0 || journals < 0 {
		t.Fatalf("tables emptied = %v, want both assets and journals", targets)
	}
	if images > journals {
		t.Errorf("tables emptied = %v, want assets before journals", targets)
	}

	// The reason the order matters, pinned so that rewriting the statement to
	// find its rows some other way fails here rather than silently making the
	// ordering above pointless.
	if !strings.Contains(db.calls[images].query, "FROM journals") {
		t.Errorf("the image delete does not find its rows through journals:\n%s", db.calls[images].query)
	}
}

// The keys have to be read while the journals rows are still there, for the
// same reason the delete has to run first: the join is the only thing that
// knows which objects were this character's. Once they are gone the bucket
// holds them and nothing can name them.
func TestTheImageKeysAreReadThroughTheJournalsTable(t *testing.T) {
	app, db := newPanelApp(1)

	_, err := app.Queries.ListCharacterJournalImages(context.Background(), queries.ListCharacterJournalImagesParams{
		CharacterID: testCharacterID,
		OwnerID:     testOwnerID,
	})
	if err != errNoRowsToGive {
		t.Fatalf("ListCharacterJournalImages error = %v, want %v", err, errNoRowsToGive)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "JOIN journals") {
		t.Fatalf("the image read does not go through journals:\n%s", call.query)
	}
	if !strings.Contains(call.query, "file_path") {
		t.Errorf("the image read returns no key to delete:\n%s", call.query)
	}
	for i, want := range []ulid.ULID{testCharacterID, testOwnerID} {
		if got, ok := call.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("arg %d = %v, want %v", i, call.args[i], want)
		}
	}
}

// EVERY STATEMENT IN THE PURGE IS SCOPED TO THE OWNER. They are the only
// deletes in the app that name no row id -- they empty a table by character --
// so the owner is the whole of the ownership check. One missing would let a
// request naming a stranger's character id empty that stranger's table.
func TestTheCharacterPurgeIsScopedToItsOwner(t *testing.T) {
	app, db := newPanelApp(1)

	if err := app.deleteCharacterRows(context.Background(), testCharacterID, testOwnerID); err != nil {
		t.Fatalf("deleteCharacterRows: %v", err)
	}

	for i, call := range db.calls {
		seen := []string{}
		for _, arg := range call.args {
			id, ok := arg.(ulid.ULID)
			if !ok {
				t.Fatalf("statement %d takes a %T, want only ULIDs:\n%s", i, arg, call.query)
			}
			switch id {
			case testOwnerID:
				seen = append(seen, "owner")
			case testCharacterID:
				seen = append(seen, "character")
			default:
				t.Fatalf("statement %d takes %v, which is neither the character nor the owner", i, id)
			}
		}

		sort.Strings(seen)
		unique := slices.Compact(seen)
		if strings.Join(unique, ",") != "character,owner" {
			t.Errorf("statement %d is scoped by %v, want the character and the owner:\n%s", i, unique, call.query)
		}
	}
}

// THE BAR'S INITIATIVE IS NOT THE COLUMN. initiative_bonus is what items and
// feats add; what a player rolls is that plus their Dexterity modifier, and a
// bar showing either half on its own would be showing a number nobody uses.
func TestTheBarsInitiativeAddsDexterityToTheStoredBonus(t *testing.T) {
	for _, c := range []struct {
		name  string
		dex   uint8
		bonus int16
		want  string
	}{
		{name: "modifier alone", dex: 16, bonus: 0, want: "+3"},
		{name: "modifier and bonus", dex: 16, bonus: 2, want: "+5"},
		{name: "a dump stat carries its sign", dex: 8, bonus: 0, want: "-1"},
		{name: "they can cancel out", dex: 8, bonus: 1, want: "+0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			character := testCharacter()
			character.Dex = c.dex
			character.InitiativeBonus = c.bonus

			if got := characterHeader(character).Initiative; got != c.want {
				t.Errorf("initiative = %q, want %q", got, c.want)
			}
		})
	}
}

// The bar reads the six columns it prints, and the speed falls back the way the
// Core Stats field does rather than rendering an empty chip.
func TestTheBarReadsItsChipsOffTheRow(t *testing.T) {
	character := testCharacter()
	character.AC = 15
	character.CurrentHP = 31
	character.MaxHP = 38
	character.ProficiencyBonus = 3

	header := characterHeader(character)

	for _, c := range []struct{ name, got, want string }{
		{"armour class", header.AC, "15"},
		{"current hit points", header.CurrentHP, "31"},
		{"hit point maximum", header.MaxHP, "38"},
		{"proficiency", header.Proficiency, "+3"},
		{"speed falls back", header.Speed, "30 ft."},
		{"passive perception", header.Passive, "14"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// The subtitle is what the row has, in order, and nothing standing in for what
// it has not -- including the alignment nobody chose.
func TestTheSubtitleOmitsWhatTheCharacterHasNot(t *testing.T) {
	for _, c := range []struct {
		name      string
		race      string
		classes   string
		alignment string
		want      string
	}{
		{
			name: "all of it", race: "Tiefling", classes: "Warlock 5", alignment: "chaotic good",
			want: "Tiefling · Warlock 5 · Soldier · Chaotic Good",
		},
		{
			name: "unaligned is not an alignment", race: "Tiefling", classes: "Warlock 5", alignment: "unaligned",
			want: "Tiefling · Warlock 5 · Soldier",
		},
		{
			name: "a blank column leaves no gap", classes: "Warlock 5", alignment: "unaligned",
			want: "Warlock 5 · Soldier",
		},
		{
			name: "a sheet with nothing on it says nothing", want: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			character := testCharacter()
			character.Race = nullString(c.race)
			character.Classes = nullString(c.classes)
			character.Alignment = nullString(c.alignment)
			if c.want != "" {
				character.Background = nullString("Soldier")
			}

			if got := characterSubtitle(character); got != c.want {
				t.Errorf("subtitle = %q, want %q", got, c.want)
			}
		})
	}
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
