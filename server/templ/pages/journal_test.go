package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func journalList(t *testing.T, entries ...JournalEntry) string {
	t.Helper()
	var buf bytes.Buffer
	data := JournalPageData{CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Query: "ring", Entries: entries}
	if err := JournalEntriesFragment(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func journalHit() JournalEntry {
	return JournalEntry{
		ID:      "01BX5ZZKBKACTAV9WEVGEMMVS1",
		Title:   "Session 12",
		Snippet: JournalSnippet{Before: "…buried the ", Match: "ring", After: " under the oak…"},
	}
}
func TestASearchResultShowsTheLineThatMatched(t *testing.T) {
	markup := journalList(t, journalHit())
	if !strings.Contains(markup, "…buried the ") || !strings.Contains(markup, " under the oak…") {
		t.Errorf("the context around the match is missing\n%s", markup)
	}
	if !strings.Contains(markup, ">ring</mark>") {
		t.Errorf("the match is not marked\n%s", markup)
	}
}
func TestTheSnippetHasNoSeamsAroundTheMark(t *testing.T) {
	markup := journalList(t, journalHit())
	if !strings.Contains(markup, `…buried the <mark`) {
		t.Errorf("whitespace was written before the mark\n%s", markup)
	}
	if !strings.Contains(markup, `</mark> under the oak…`) {
		t.Errorf("whitespace was written after the mark\n%s", markup)
	}
}
func TestASnippetCannotCarryMarkupOutOfAnEntry(t *testing.T) {
	markup := journalList(t, JournalEntry{
		ID:    "01BX5ZZKBKACTAV9WEVGEMMVS1",
		Title: "Session 12",
		Snippet: JournalSnippet{
			Before: `<script>alert(1)</script>`,
			Match:  `<img src=x onerror=alert(1)>`,
			After:  `</mark><b>after</b>`,
		},
	})
	for _, tag := range []string{"<script", "<img", "<b>"} {
		if strings.Contains(markup, tag) {
			t.Errorf("%q reached the page as a tag\n%s", tag, markup)
		}
	}
	if strings.Count(markup, "<mark") != 1 || strings.Count(markup, "</mark>") != 1 {
		t.Errorf("the entry closed or opened a mark of its own\n%s", markup)
	}
	if !strings.Contains(markup, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Errorf("the matched text was not shown as its own characters\n%s", markup)
	}
}
func TestAnEntryWithNoSnippetRendersNoLine(t *testing.T) {
	markup := journalList(t, JournalEntry{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Title: "The ring"})
	if strings.Contains(markup, "<mark") {
		t.Errorf("an empty snippet rendered a mark\n%s", markup)
	}
	if strings.Contains(markup, "line-clamp-2") {
		t.Errorf("an empty snippet rendered its line\n%s", markup)
	}
}
func TestTheUnfilteredListCarriesNoSnippets(t *testing.T) {
	var buf bytes.Buffer
	data := JournalPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Entries:     []JournalEntry{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Title: "Session 12"}},
	}
	if err := JournalEntriesFragment(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "<mark") {
		t.Errorf("the unfiltered list rendered a snippet\n%s", buf.String())
	}
}

const journalRoomID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

var rendered = strings.NewReplacer("&", "&amp;", `"`, "&#34;")

func journalWindow(t *testing.T, entries ...JournalEntry) string {
	t.Helper()
	return renderJournal(t, CharacterJournalWindow(JournalPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		RoomID:      journalRoomID,
		Entries:     entries,
	}))
}
func renderJournal(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func TestAnEntryOpenedAtTheTableStaysInTheWindow(t *testing.T) {
	markup := journalWindow(t, journalHit())
	if strings.Contains(markup, `href="/characters/`) {
		t.Errorf("the window sends the player out of the room to read an entry\n%s", markup)
	}
	want := rendered.Replace(JournalEntryWindowPath(journalRoomID, "01BX5ZZKBKACTAV9WEVGEMMVS1"))
	if strings.Count(markup, `hx-get="`+want+`"`) != 2 {
		t.Errorf("the title and the open button do not both load %q\n%s", want, markup)
	}
	if !strings.Contains(markup, `hx-target="#`+JournalBodyID+`"`) {
		t.Errorf("an entry is not swapped into the window body\n%s", markup)
	}
}
func TestTheJournalPageStillLinksToItsEntries(t *testing.T) {
	markup := journalList(t, journalHit())
	if !strings.Contains(markup, `href="/characters/01ARZ3NDEKTSV4RRFFQ69G5FAV/edit/journal/01BX5ZZKBKACTAV9WEVGEMMVS1"`) {
		t.Errorf("the journal page no longer links to the entry it lists\n%s", markup)
	}
}
func TestTheWindowWritesANewEntryWithoutLeavingTheRoom(t *testing.T) {
	markup := journalWindow(t)
	if !strings.Contains(markup, `hx-post="/characters/01ARZ3NDEKTSV4RRFFQ69G5FAV/journal"`) {
		t.Errorf("the window offers no way to start an entry\n%s", markup)
	}
	if !strings.Contains(markup, rendered.Replace(journalRoomVals(journalRoomID))) {
		t.Errorf("the new entry does not name the room it was written in\n%s", markup)
	}
	if strings.Contains(markup, "<form") {
		t.Errorf("a form in a window posts the page away from the table\n%s", markup)
	}
}
func TestTheWindowSearchesItselfAndThePageSearchesTheList(t *testing.T) {
	if got := journalSearchPath(JournalPageData{CharacterID: "c", RoomID: journalRoomID}); got != JournalWindowPath(journalRoomID) {
		t.Errorf("the window searches %q", got)
	}
	if got := journalSearchPath(JournalPageData{CharacterID: "c"}); got != "/fragment/character/journal-entries?character=c" {
		t.Errorf("the page searches %q", got)
	}
}
func TestOnlyTheWindowCarriesTheEditorScript(t *testing.T) {
	window := renderJournal(t, CharacterJournalWindowEntry(JournalEntryPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		RoomID:      journalRoomID,
		EntryID:     "01BX5ZZKBKACTAV9WEVGEMMVS1",
	}))
	if !strings.Contains(window, `data-journal-editor-src="`+JournalEditorScript+`"`) {
		t.Errorf("the window cannot find the editor to load\n%s", window)
	}
	if !strings.Contains(window, `hx-get="`+JournalWindowPath(journalRoomID)+`"`) {
		t.Errorf("there is no way back to the entry list\n%s", window)
	}
	page := renderJournal(t, EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EntryID:     "01BX5ZZKBKACTAV9WEVGEMMVS1",
	}))
	if strings.Contains(page, "data-journal-editor-src") {
		t.Errorf("the page asks for the editor twice; it already loads it as a script")
	}
	if !strings.Contains(page, `src="`+JournalEditorScript+`"`) {
		t.Errorf("the entry page no longer loads the editor\n%s", page)
	}
}

func journalWindowEntry(t *testing.T) string {
	t.Helper()
	return renderJournal(t, CharacterJournalWindowEntry(JournalEntryPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		RoomID:      journalRoomID,
		EntryID:     "01BX5ZZKBKACTAV9WEVGEMMVS1",
	}))
}
func TestTheWindowEntryWearsNoPanelOfItsOwn(t *testing.T) {
	window := journalWindowEntry(t)
	for _, chrome := range []string{surfacePanel, "card-body"} {
		if strings.Contains(window, chrome) {
			t.Errorf("a floating panel sits inside the window, which is already one: %q\n%s", chrome, window)
		}
	}
	page := renderJournal(t, EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		EntryID:     "01BX5ZZKBKACTAV9WEVGEMMVS1",
	}))
	if !strings.Contains(page, "card-body") || !strings.Contains(page, surfacePanel) {
		t.Errorf("the entry page lost the panel the window does not want\n%s", page)
	}
}
func TestSavingSitsRightOfTheWayBack(t *testing.T) {
	window := journalWindowEntry(t)
	back := strings.Index(window, "Back to all entries")
	save := strings.Index(window, ">Save</button>")
	if back < 0 || save < 0 {
		t.Fatalf("the window is missing its way back or its save\n%s", window)
	}
	if save < back {
		t.Errorf("save is written before the way back, so it reads first\n%s", window)
	}
	if !strings.Contains(window, `class="btn btn-primary btn-sm ml-auto"`) {
		t.Errorf("save is not the primary button pushed to the right\n%s", window)
	}
	if !strings.Contains(window, `d="M5 12l6 -6"`) {
		t.Errorf("the way back carries no arrow\n%s", window)
	}
}
