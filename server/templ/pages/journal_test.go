package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
