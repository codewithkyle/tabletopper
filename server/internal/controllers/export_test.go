package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/session"
)

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
func TestTheSharedExportURLIsTheSharesOwn(t *testing.T) {
	if got := shareExportURL("tok"); got != "/share/tok/export.md" {
		t.Errorf("shareExportURL = %q", got)
	}
}
