package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// journalList renders the entries fragment, which is what both the page and the
// search route put on screen.
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

// A result says why it matched. Without this the card is a title and two dates,
// and an entry that matched on a word in its body is indistinguishable from one
// that matched on nothing.
func TestASearchResultShowsTheLineThatMatched(t *testing.T) {
	markup := journalList(t, journalHit())

	if !strings.Contains(markup, "…buried the ") || !strings.Contains(markup, " under the oak…") {
		t.Errorf("the context around the match is missing\n%s", markup)
	}
	if !strings.Contains(markup, ">ring</mark>") {
		t.Errorf("the match is not marked\n%s", markup)
	}
}

// The three fields are joined with nothing between them, so the sentence reads
// as one line rather than as three with gaps at the seams. templ decides
// whether to write whitespace between an expression and the tag beside it, and
// the answer has to be no.
func TestTheSnippetHasNoSeamsAroundTheMark(t *testing.T) {
	markup := journalList(t, journalHit())

	if !strings.Contains(markup, `…buried the <mark`) {
		t.Errorf("whitespace was written before the mark\n%s", markup)
	}
	if !strings.Contains(markup, `</mark> under the oak…`) {
		t.Errorf("whitespace was written after the mark\n%s", markup)
	}
}

// THE SNIPPET IS CUT STRAIGHT OUT OF A JOURNAL BODY, which is the least trusted
// text in the app -- internal/markdown exists to render one safely and none of
// it is in the way here. Three plain strings are what keeps that safe: templ
// escapes them like every other value, so the only markup a snippet can produce
// is markup the template wrote. A snippet assembled into one string of HTML
// would have made this the second place in the app writing a body out raw.
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

	// Escaped rather than dropped: the reader still sees what the entry says,
	// as the words it is rather than as the markup it looks like.
	if !strings.Contains(markup, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Errorf("the matched text was not shown as its own characters\n%s", markup)
	}
}

// An entry that matched on its title alone carries no snippet, and the line is
// left out rather than rendered empty. The term is in the heading an inch
// above; printing it again underneath would be the same words twice.
func TestAnEntryWithNoSnippetRendersNoLine(t *testing.T) {
	markup := journalList(t, JournalEntry{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Title: "The ring"})

	if strings.Contains(markup, "<mark") {
		t.Errorf("an empty snippet rendered a mark\n%s", markup)
	}
	if strings.Contains(markup, "line-clamp-2") {
		t.Errorf("an empty snippet rendered its line\n%s", markup)
	}
}

// The unfiltered list has no term and reads no bodies, so every card on it is
// the card this page has always rendered.
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
