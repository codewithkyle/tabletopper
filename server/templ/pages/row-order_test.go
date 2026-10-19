package pages

import (
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestAttackAndSpellRowsMoveByTheirHandle(t *testing.T) {
	attacks := renderString(t, attacksTable(sheetCharacterID, []Attack{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Longsword"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Name: "Shortbow"},
	}))
	spells := renderString(t, spellsSection(SpellLevelPageData{
		CharacterID: sheetCharacterID,
		Level:       3,
		Current:     testSpellCounters(3),
		Spells: []Spell{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS2", Level: 3, Name: "Fireball", School: DefaultSpellSchool},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS3", Level: 3, Name: "Counterspell", School: "Abjuration"},
		},
	}))
	for _, c := range []struct {
		name  string
		body  string
		order string
		rows  [][2]string
	}{
		{"attack", attacks, "/characters/" + sheetCharacterID + "/attacks/order", [][2]string{
			{"01BX5ZZKBKACTAV9WEVGEMMVS0", "Longsword"},
			{"01BX5ZZKBKACTAV9WEVGEMMVS1", "Shortbow"},
		}},
		{"spell", spells, "/characters/" + sheetCharacterID + "/spells/order/3", [][2]string{
			{"01BX5ZZKBKACTAV9WEVGEMMVS2", "Fireball"},
			{"01BX5ZZKBKACTAV9WEVGEMMVS3", "Counterspell"},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			save := `<button id="` + c.name + `-order" type="button" hidden hx-post="` + c.order + `" hx-swap="none" hx-vals="">`
			if !strings.Contains(c.body, `data-sort-list="`+c.name+`-order"`) || !strings.Contains(c.body, save) {
				t.Errorf("the list does not name a hidden button that saves to %s\n%s", c.order, c.body)
			}
			if strings.Index(c.body, save) < strings.Index(c.body, "data-sort-list") {
				t.Error("the save button sits inside the list, where the empty note can never be the only child")
			}
			at := strings.Index(c.body, "only:block")
			if at < 0 {
				t.Fatal("the list says nothing when it is empty")
			}
			for _, row := range c.rows {
				next := strings.Index(c.body, `data-sort-row="`+row[0]+`"`)
				if next < at {
					t.Errorf("%s is out of order, or ahead of the empty note a new row is added after", row[1])
				}
				at = next
				if !strings.Contains(c.body, `aria-label="Move `+row[1]+`"`) {
					t.Errorf("%s's handle does not say what it moves", row[1])
				}
			}
			if got := strings.Count(c.body, "data-sort-handle"); got != len(c.rows) {
				t.Errorf("handles = %d, want one per row", got)
			}
		})
	}
}
func TestOnlyAJustAddedRowTakesTheCaret(t *testing.T) {
	for name, rows := range map[string][2]templ.Component{
		"attack": {
			AttackRow(sheetCharacterID, Attack{ID: testAttackRowID, Fresh: true}),
			AttackRow(sheetCharacterID, Attack{ID: testAttackRowID}),
		},
		"spell": {
			SpellRow(sheetCharacterID, Spell{ID: testAttackRowID, Level: 1, School: DefaultSpellSchool, Fresh: true}),
			SpellRow(sheetCharacterID, Spell{ID: testAttackRowID, Level: 1, School: DefaultSpellSchool}),
		},
	} {
		fresh, stored := renderString(t, rows[0]), renderString(t, rows[1])
		if got := strings.Count(fresh, " autofocus"); got != 1 {
			t.Errorf("a new %s row autofocuses %d fields, want its name", name, got)
		}
		if field := strings.Index(fresh, `name="name"`); field < 0 || strings.Index(fresh, " autofocus") < field {
			t.Errorf("the new %s row does not focus its name", name)
		}
		if strings.Contains(stored, "autofocus") {
			t.Errorf("a stored %s row takes the caret on every page load", name)
		}
	}
}
