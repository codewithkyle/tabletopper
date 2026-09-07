package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/session"
)

// The export routes, bounded the way every other test here that touches a share
// is: recordingDB answers a :one by failing, so neither file can actually be
// built from this harness. What is covered is the half that decides who gets
// one -- the ids the reads are scoped by, the answer to a request that names
// nothing, and the headers that make the reply a download rather than a page.

func exportRequest(t *testing.T, handler http.HandlerFunc, pathValues map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/export.md", nil)
	for key, value := range pathValues {
		r.SetPathValue(key, value)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

// An id that will not parse never becomes a statement, and the answer is a
// navigation rather than an alert: this route is an anchor somebody clicked, so
// there is no page open to show an alert in and nothing to swap it into.
func TestAnExportOfSomethingUnnamedRunsNoStatements(t *testing.T) {
	for name, c := range map[string]struct {
		handler func(*App) http.HandlerFunc
		want    string
	}{
		"monster":   {func(a *App) http.HandlerFunc { return a.ExportMonster }, "/monsters"},
		"character": {func(a *App) http.HandlerFunc { return a.ExportCharacter }, "/characters"},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := exportRequest(t, c.handler(app), map[string]string{"id": "not-a-ulid"})

			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); got != c.want {
				t.Errorf("sent to %q, want %q", got, c.want)
			}
			if len(db.calls) != 0 || len(db.reads) != 0 {
				t.Error("an unparseable id reached the database")
			}
		})
	}
}

// A token that is not shaped like one of ours cannot name a row, so it is
// refused before anything is queried -- and it is the same flat dead-link page
// every other miss on a share gets, because telling them apart would tell
// somebody guessing which half of the guess was right.
func TestASharedExportWithABadTokenRunsNoStatements(t *testing.T) {
	app, db := newPanelApp(1)

	rec := exportRequest(t, app.ExportShare, map[string]string{"token": "nope"})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 || len(db.reads) != 0 {
		t.Error("a malformed token reached the database")
	}
}

// BOTH EXPORTS READ WITH THE IDS THEY WERE HANDED AND NOTHING ELSE. It is the
// property the shared export rests on: the file is built from the share row's
// monster and the share row's owner, so a reader downloading somebody else's
// stat block is reading it as its owner rather than as themselves -- and a read
// that reached for the session's user instead would find nothing at all.
func TestAnExportReadsExactlyTheThingItWasAskedFor(t *testing.T) {
	for name, c := range map[string]struct {
		export func(*App) error
		table  string
	}{
		"monster": {func(a *App) error {
			_, _, err := a.monsterMarkdown(context.Background(), testMonsterID, testOwnerID)
			return err
		}, "FROM monsters"},
		"character": {func(a *App) error {
			_, _, err := a.characterMarkdown(context.Background(), testCharacterID, testOwnerID)
			return err
		}, "FROM characters"},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)

			if err := c.export(app); err == nil {
				t.Fatal("the export succeeded against a database that answers no reads")
			}
			if len(db.reads) == 0 {
				t.Fatal("the export read nothing")
			}

			read := db.reads[0]
			if !strings.Contains(read.query, c.table) {
				t.Errorf("the first read is not the thing being exported:\n%s", read.query)
			}
			if len(read.args) != 2 {
				t.Fatalf("the read takes %d arguments, want the thing and its owner", len(read.args))
			}
			if owner, ok := boundID(read.args[1]); !ok || owner != testOwnerID {
				t.Errorf("the export is not scoped to the owner it was given: %v", read.args)
			}
		})
	}
}

// Content-Disposition IS WHAT MAKES IT A DOWNLOAD rather than a page of plain
// text in a tab, and the filename in it is what the file lands as. no-store
// because two of the four routes are behind a session and a third is behind a
// password, and a shared cache holding one reader's answer would serve it to
// the next.
func TestAMarkdownExportIsSentAsAFileAndNotAsAPage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeMarkdown(rec, []byte("# Goblin\n"), "goblin.md")

	for header, want := range map[string]string{
		"Content-Type":        "text/markdown; charset=utf-8",
		"Content-Disposition": `attachment; filename="goblin.md"`,
		"Cache-Control":       "no-store",
		"X-Robots-Tag":        "noindex, nofollow",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.String() != "# Goblin\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// The download a shared page offers is under the share's own prefix, like the
// portrait and the import: a reader is handed one token and everything they can
// reach hangs off it.
func TestTheSharedExportURLIsTheSharesOwn(t *testing.T) {
	if got := shareExportURL("tok"); got != "/share/tok/export.md" {
		t.Errorf("shareExportURL = %q", got)
	}
}
