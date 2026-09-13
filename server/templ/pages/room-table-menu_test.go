package pages

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func testRing(n int) []RoomTableMenuArt {
	out := make([]RoomTableMenuArt, 0, n)
	for i := range n {
		out = append(out, RoomTableMenuArt{
			ID:    "01ART" + strconv.Itoa(i),
			Name:  "Pines " + strconv.Itoa(i),
			Image: "/assets/images/01ART" + strconv.Itoa(i),
			Index: i,
			Count: n,
		})
	}
	return out
}
func testWheel(n int) RoomTableMenuData {
	return NewTableMenu(testTableRoomID, true, testRing(n))
}
func styleOf(t *testing.T, markup string, property string) []string {
	t.Helper()
	found := regexp.MustCompile(property+`[:=] *"?([^;"]+)`).FindAllStringSubmatch(markup, -1)
	out := make([]string, 0, len(found))
	for _, m := range found {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}
func TestTheRingCarriesOneButtonPerPaletteEntry(t *testing.T) {
	page := renderToString(t, RoomTableMenu(testWheel(3)))
	if got := strings.Count(page, "data-table-menu-art="); got != 3 {
		t.Fatalf("the ring carries %d pictures, want one per palette entry:\n%s", got, page)
	}
	for _, want := range []string{`data-table-menu-art="01ART0"`, `alt=""`, "/assets/images/01ART1", "Pines 2"} {
		if !strings.Contains(page, want) {
			t.Errorf("the ring is missing %s:\n%s", want, page)
		}
	}
}
func TestEverythingOnTheWheelSitsOnOneRing(t *testing.T) {
	page := renderToString(t, RoomTableMenu(testWheel(2)))
	radii := styleOf(t, page, "--radius")
	if len(radii) != 4 {
		t.Fatalf("%d buttons carry a radius, want two actions and two pictures", len(radii))
	}
	for _, radius := range radii {
		if radius != radii[0] {
			t.Fatalf("the wheel's buttons sit at %v, want one ring", radii)
		}
	}
	degrees := styleOf(t, page, "--degree")
	if len(degrees) != 4 || degrees[0] != "90deg" {
		t.Fatalf("the wheel's degrees are %v, want four starting at the top", degrees)
	}
	if degrees[1] != "0deg" || degrees[2] != "-90deg" || degrees[3] != "-180deg" {
		t.Errorf("the wheel's degrees are %v, want a quarter turn between each", degrees)
	}
}
func TestTheActionsComeFirstSoTheyStayWhereTheGMLeftThem(t *testing.T) {
	two := styleOf(t, renderToString(t, RoomTableMenu(testWheel(2))), "--degree")
	twenty := styleOf(t, renderToString(t, RoomTableMenu(testWheel(20))), "--degree")
	if two[0] != twenty[0] {
		t.Errorf("the first action moved from %s to %s when the bag grew", two[0], twenty[0])
	}
}
func TestAFullWheelIsPushedOutUntilItsPicturesFit(t *testing.T) {
	page := renderToString(t, RoomTableMenu(testWheel(24)))
	want := strconv.Itoa(int(math.Round(26*48/(2*math.Pi)))) + "px"
	got := styleOf(t, page, "--radius")
	if len(got) != 26 {
		t.Fatalf("%d buttons carry a radius, want 24 pictures and two actions", len(got))
	}
	if got[0] != want {
		t.Errorf("a full wheel sits at %s, want %s -- any tighter and the buttons overlap", got[0], want)
	}
}
func TestTheHubHoldsTheActionsAndTheRingHoldsThePictures(t *testing.T) {
	page := renderToString(t, RoomTableMenu(testWheel(2)))
	for _, want := range []string{tableMenuPartyLabel, tableMenuEraseLabel} {
		if !strings.Contains(page, `data-tip="`+want+`"`) {
			t.Errorf("the wheel has no %q:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "data-table-menu-erase") {
		t.Error("the eraser is not reachable from the wheel")
	}
	if strings.Contains(page, "data-table-menu-label") {
		t.Error("the wheel still carries a place to write words over the table")
	}
}
func TestTheWheelIsPicturesAndIconsAndNoWordsOverTheTable(t *testing.T) {
	page := renderToString(t, RoomTableMenu(testWheel(2)))
	body := regexp.MustCompile(`>[^<]+<`).FindAllString(page, -1)
	for _, text := range body {
		if strings.TrimSpace(strings.Trim(text, "><")) != "" {
			t.Errorf("the wheel writes %q over the table; it is pictures and icons", text)
		}
	}
	if strings.Contains(page, "data-hint=") {
		t.Error("a button still carries prose for the wheel to render")
	}
	for _, want := range []string{`aria-label="Pines 0"`, `aria-label="` + tableMenuEraseLabel + `"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the wheel is missing %s, so a screen reader gets nothing:\n%s", want, page)
		}
	}
}
func TestAGMWithAnEmptyBagStillGetsTheHub(t *testing.T) {
	page := renderToString(t, RoomTableMenu(NewTableMenu(testTableRoomID, true, nil)))
	if !strings.Contains(page, "data-table-menu") {
		t.Fatalf("a GM with nothing in the bag lost the wheel:\n%s", page)
	}
	if strings.Contains(page, "data-table-menu-art=") {
		t.Error("an empty bag still rendered a ring")
	}
	if strings.Contains(page, "data-table-menu-erase") {
		t.Error("there is an eraser with nothing to erase with")
	}
}
func TestTheWheelReachesPastWhicheverRingItHas(t *testing.T) {
	bare, err := strconv.Atoi(NewTableMenu(testTableRoomID, true, nil).Reach())
	if err != nil {
		t.Fatalf("the reach is not a number: %v", err)
	}
	full, err := strconv.Atoi(testWheel(24).Reach())
	if err != nil {
		t.Fatalf("the reach is not a number: %v", err)
	}
	if full <= bare {
		t.Errorf("a wheel with a full ring reaches %d, no further than a bare hub at %d", full, bare)
	}
}
