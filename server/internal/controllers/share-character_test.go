package controllers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)

func TestASharedSheetHoldsEverySpellByLevelWithTheSlotsLeft(t *testing.T) {
	book := sharedSpellBook([]queries.Spell{
		{Level: 0, Name: "Druidcraft"},
		{Level: 1, Name: "Ensnaring Strike"},
		{Level: 1, Name: "Hunter's Mark", IsPrepared: true, School: "Divination", CastingTime: "Bonus action", CastingRange: "90 feet", Duration: "1 hour", Components: "V", Description: "Mark it."},
	}, []pages.SpellLevel{
		{Level: 0, Slots: "0", Used: "0", Count: 1},
		{Level: 1, Slots: "4", Used: "1", Count: 2},
		{Level: 2, Slots: "0", Used: "0", Count: 0},
		{Level: 3, Slots: "2", Used: "2", Count: 0},
	})
	want := []struct {
		name   string
		slots  string
		spells int
	}{
		{"Cantrips", "Unlimited", 1},
		{"Level 1", "3 of 4 slots left", 2},
		{"Level 3", "0 of 2 slots left", 0},
	}
	if len(book) != len(want) {
		t.Fatalf("the book has %d levels, want %d: %+v", len(book), len(want), book)
	}
	for i, w := range want {
		if book[i].Name != w.name || book[i].Slots != w.slots || len(book[i].Spells) != w.spells {
			t.Errorf("level %d = %s %q with %d spells, want %s %q with %d", i, book[i].Name, book[i].Slots, len(book[i].Spells), w.name, w.slots, w.spells)
		}
	}
	mark := book[1].Spells[1]
	if !mark.Prepared || mark.School != "Divination" || mark.CastingTime != "Bonus action" || mark.Range != "90 feet" ||
		mark.Duration != "1 hour" || mark.Components != "V" || mark.Description != "Mark it." {
		t.Errorf("a spell lost some of itself on the way to the page: %+v", mark)
	}
	if book[1].Spells[0].Prepared {
		t.Error("an unprepared spell reads as prepared")
	}
}
func TestASharedSheetHoldsTheWholeInventory(t *testing.T) {
	items := sharedItems([]queries.Inventory{
		{Name: "Longbow", Quantity: 1, Value: "50 gp", Weight: 2, Equipped: true},
		{Name: "Hempen Rope", Quantity: 2, Value: "1 gp", Weight: 10.5, Description: "Knotted."},
		{Quantity: 1},
	})
	want := []pages.SharedItem{
		{Name: "Longbow", Quantity: "1", Value: "50 gp", Weight: "2", Equipped: true},
		{Name: "Hempen Rope", Quantity: "2", Value: "1 gp", Weight: "10.5", Description: "Knotted."},
		{Name: "Unnamed item", Quantity: "1"},
	}
	if len(items) != len(want) {
		t.Fatalf("%d items shared, want every one of %d", len(items), len(want))
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, items[i], want[i])
		}
	}
}
func TestASharedSheetSaysWhereTheCharacterIsTrained(t *testing.T) {
	bonuses := sharedBonuses([]pages.BonusRow{
		{Label: "Stealth", Proficiency: pages.ProficiencyExpertise, Total: "+9"},
		{Label: "Athletics", Proficiency: pages.ProficiencyNone, Total: "+1"},
		{Label: "History", Proficiency: pages.ProficiencyHalf, Total: "+2"},
	})
	for i, want := range []string{"Expertise", "", "Half"} {
		if bonuses[i].Proficiency != want {
			t.Errorf("%s reads %q, want %q", bonuses[i].Label, bonuses[i].Proficiency, want)
		}
	}
}
func TestTheSharedSpellListIsEveryLevelAndEverySpell(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "spells.sql"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "-- name: ListCharacterSpells")
	if start < 0 {
		t.Fatal("no ListCharacterSpells query")
	}
	statement := body[start:]
	if end := strings.Index(statement[1:], "-- name:"); end >= 0 {
		statement = statement[:end+1]
	}
	for _, narrowing := range []string{"is_prepared", "level = ?"} {
		if strings.Contains(statement, narrowing) {
			t.Errorf("the shared spell list is narrowed by %s:\n%s", narrowing, statement)
		}
	}
	if !strings.Contains(statement, "ORDER BY level") {
		t.Errorf("the shared spell list is not in level order:\n%s", statement)
	}
}
