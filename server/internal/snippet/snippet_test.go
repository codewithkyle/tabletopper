package snippet
import (
	"strings"
	"testing"
	"unicode/utf8"
)
const prose = "We spent the morning in the market square before the guards moved us on, " +
	"and by evening had found Béornegar drinking alone at the Crooked Lantern."
func TestAnUnaccentedSearchFindsTheAccentedName(t *testing.T) {
	hit, ok := Find(prose, "Beornegar", 60)
	if !ok {
		t.Fatal("the accented name was not found by its plain spelling")
	}
	if hit.Match != "Béornegar" {
		t.Errorf("marked %q, want the writer's own spelling", hit.Match)
	}
}
func TestTheMarkedTextIsTheWritersSpelling(t *testing.T) {
	hit, ok := Find("The Crooked Lantern was shut.", "crooked lantern", 40)
	if !ok {
		t.Fatal("a lower-case search did not find the capitalised name")
	}
	if hit.Match != "Crooked Lantern" {
		t.Errorf("marked %q, want %q", hit.Match, "Crooked Lantern")
	}
}
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
func TestATermThatIsNotInTheTextIsNotAHit(t *testing.T) {
	if _, ok := Find(prose, "assets", 60); ok {
		t.Error("a word the entry does not contain was reported as a hit")
	}
}
func TestAnEmptyTermIsNotAHit(t *testing.T) {
	for _, term := range []string{"", "   "} {
		if _, ok := Find(prose, term, 60); ok {
			t.Errorf("the term %q was reported as a hit", term)
		}
	}
}
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
func TestAMultiWordTermMatchesAsAPhrase(t *testing.T) {
	hit, ok := Find(prose, "market square", 40)
	if !ok {
		t.Fatal("a two-word term was not found")
	}
	if hit.Match != "market square" {
		t.Errorf("marked %q, want the whole phrase", hit.Match)
	}
}
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
