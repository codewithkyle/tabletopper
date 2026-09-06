package snippet

import (
	"strings"
	"testing"
	"unicode/utf8"
)

const prose = "We spent the morning in the market square before the guards moved us on, " +
	"and by evening had found Béornegar drinking alone at the Crooked Lantern."

// The property the whole package exists to keep. journals is utf8mb4_0900_ai_ci,
// so MySQL has already matched the unaccented spelling by the time a row is
// handed here; a fold that did less would drop a row the database was right to
// return, and the reader would watch a result disappear as they typed the name
// the way they heard it.
func TestAnUnaccentedSearchFindsTheAccentedName(t *testing.T) {
	hit, ok := Find(prose, "Beornegar", 60)
	if !ok {
		t.Fatal("the accented name was not found by its plain spelling")
	}
	if hit.Match != "Béornegar" {
		t.Errorf("marked %q, want the writer's own spelling", hit.Match)
	}
}

// The marked text is the entry's, not the query's, in case as well as accent.
func TestTheMarkedTextIsTheWritersSpelling(t *testing.T) {
	hit, ok := Find("The Crooked Lantern was shut.", "crooked lantern", 40)
	if !ok {
		t.Fatal("a lower-case search did not find the capitalised name")
	}
	if hit.Match != "Crooked Lantern" {
		t.Errorf("marked %q, want %q", hit.Match, "Crooked Lantern")
	}
}

// THE TEST THAT CATCHES THE FOLD WRITTEN THE OBVIOUS WAY. Every accent before
// the match makes the folded string shorter than the original by a byte, so an
// offset measured in the fold and applied to the original lands earlier and
// earlier -- and the further in the match is, the further the cut slides. A
// fold that returns no offset map passes every other test here and fails this
// one by marking the wrong word or by splitting a rune in half.
func TestOffsetsSurviveTheAccentsBeforeTheMatch(t *testing.T) {
	body := strings.Repeat("Béornegar and Ísolde and Æthelred walked. ", 12) + "The ring was buried here."

	hit, ok := Find(body, "ring", 30)
	if !ok {
		t.Fatal("the term was not found past a run of accented names")
	}
	if hit.Match != "ring" {
		t.Errorf("marked %q, want %q -- the window slid", hit.Match, "ring")
	}
	for _, part := range []string{hit.Before, hit.Match, hit.After} {
		if !utf8.ValidString(part) {
			t.Errorf("the cut split a rune: %q", part)
		}
	}
}

// A term that is only in the markdown's plumbing is not a result. The caller
// hands over the projected body, so this is what makes the URL behind a picture
// stop matching.
func TestATermThatIsNotInTheTextIsNotAHit(t *testing.T) {
	if _, ok := Find(prose, "assets", 60); ok {
		t.Error("a word the entry does not contain was reported as a hit")
	}
}

// An empty term matches nothing rather than everything. The handler already
// routes a blank box to the unfiltered list, so this is the second line.
func TestAnEmptyTermIsNotAHit(t *testing.T) {
	for _, term := range []string{"", "   "} {
		if _, ok := Find(prose, term, 60); ok {
			t.Errorf("the term %q was reported as a hit", term)
		}
	}
}

// The ellipsis is a claim that text was cut, so it is absent when none was.
func TestTheEllipsisMarksOnlyAWindowThatCut(t *testing.T) {
	hit, ok := Find(prose, "morning", 60)
	if !ok {
		t.Fatal("the term was not found")
	}
	if strings.HasPrefix(hit.Before, "…") {
		t.Errorf("the start of the entry was marked as cut: %q", hit.Before)
	}
	if !strings.HasSuffix(hit.After, "…") {
		t.Errorf("the tail was cut and not marked: %q", hit.After)
	}

	whole, ok := Find("A short line.", "short", 60)
	if !ok {
		t.Fatal("the term was not found")
	}
	if strings.Contains(whole.Before+whole.After, "…") {
		t.Errorf("a line shorter than the window was marked as cut: %q / %q", whole.Before, whole.After)
	}
}

// The window opens on a word rather than in the middle of one, so a snippet
// reads as a phrase.
func TestTheWindowCutsAtAWordBoundary(t *testing.T) {
	hit, ok := Find(prose, "Crooked", 24)
	if !ok {
		t.Fatal("the term was not found")
	}

	before := strings.TrimPrefix(hit.Before, "…")
	if before != "" && strings.HasPrefix(before, " ") {
		t.Errorf("the window opened on a space: %q", hit.Before)
	}
	if strings.Contains(before, "vening") && !strings.Contains(before, "evening") {
		t.Errorf("the window opened inside a word: %q", hit.Before)
	}
}

// One hit per entry. An entry naming a town nine times is one result with one
// line of context, and it is the first mention -- the sentence most likely to
// be the one that introduces it.
func TestTheFirstMatchIsTheOneShown(t *testing.T) {
	hit, ok := Find("The ring is first. Then more words follow. The ring is second.", "ring", 12)
	if !ok {
		t.Fatal("the term was not found")
	}
	if !strings.Contains(hit.Before, "The ") || strings.Contains(hit.Before, "second") {
		t.Errorf("the second mention was shown: %q", hit.Before)
	}
	if !strings.Contains(hit.After, "first") {
		t.Errorf("the window did not open on the first mention: %q", hit.After)
	}
}

// A term spanning a space is one term. The box is read as ctrl-F and ctrl-F
// takes a phrase.
func TestAMultiWordTermMatchesAsAPhrase(t *testing.T) {
	hit, ok := Find(prose, "market square", 40)
	if !ok {
		t.Fatal("a two-word term was not found")
	}
	if hit.Match != "market square" {
		t.Errorf("marked %q, want the whole phrase", hit.Match)
	}
}

// Contains is what a title is checked with, and it folds the same way Find
// does -- a title is a plain column, so a match in one is always visible.
func TestContainsFoldsTheSameWayFindDoes(t *testing.T) {
	if !Contains("Session 12: Béornegar", "beornegar") {
		t.Error("a title match was missed by case and accent")
	}
	if Contains("Session 12", "") {
		t.Error("an empty term matched a title")
	}
	if Contains("Session 12", "marsh") {
		t.Error("a title matched a word it does not contain")
	}
}
