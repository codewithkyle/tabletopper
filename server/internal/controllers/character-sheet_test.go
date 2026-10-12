package controllers

import (
	"context"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

func TestTheWindowServesEveryTabThePageHas(t *testing.T) {
	for _, tab := range pages.SheetTabs() {
		if !sheetSections[tab.Section] {
			t.Errorf("the window refuses the %s tab", tab.Section)
		}
	}
	for _, section := range pages.LiveSheetSections() {
		if !sheetSections[section] {
			t.Errorf("the window refuses the live %s panel", section)
		}
	}
	if sheetSections["journal"] {
		t.Error("the window serves the journal, which the room has its own window for")
	}
}
func TestTheSpellsTabWithNoLevelIsThePreparedView(t *testing.T) {
	if level, ok := sheetLevel(""); !ok || level != pages.SpellsPrepared {
		t.Errorf("no level reads as %d (ok %v), want the prepared view", level, ok)
	}
	for raw, want := range map[string]int{"0": 0, "3": 3, "9": 9} {
		if level, ok := sheetLevel(raw); !ok || level != want {
			t.Errorf("level %q reads as %d (ok %v), want %d", raw, level, ok, want)
		}
	}
	for _, raw := range []string{"-1", "10", "prepared"} {
		if _, ok := sheetLevel(raw); ok {
			t.Errorf("level %q is accepted", raw)
		}
	}
}
func TestEachTabReadsOnlyWhatItShows(t *testing.T) {
	for section, want := range map[string]string{
		pages.SheetSectionMain:      "",
		pages.SheetSectionAbilities: "",
		pages.SheetSectionSkills:    "",
		pages.SheetSectionAttacks:   "-- name: ListCharacterAttacks",
		pages.SheetSectionSpells:    "-- name: ListPreparedSpells",
	} {
		app, db := newPanelApp(1)
		_, _ = app.sheetSectionData(context.Background(), testCharacterID, testOwnerID, queries.Character{}, section)
		if want == "" {
			if len(db.calls) != 0 {
				t.Errorf("the %s tab ran %d statements beyond the character it was handed", section, len(db.calls))
			}
			continue
		}
		if len(db.calls) == 0 || !strings.Contains(db.calls[0].query, want) {
			t.Errorf("the %s tab did not start with %s: %v", section, want, db.calls)
		}
	}
}
