package pages

import (
	"strconv"
	"strings"
	"testing"
)

const levelTwentyXP = 355_000

func characterSheet(t *testing.T) string {
	t.Helper()
	return everyEditorTab(t, EditCharacterPageData{
		XP:        "355000",
		AC:        "17",
		MaxHP:     "140",
		CurrentHP: "92",
		TempHP:    "0",
		Str:       "16",
	})
}
func fieldTag(t *testing.T, markup string, attribute string) string {
	t.Helper()
	at := strings.Index(markup, attribute)
	if at < 0 {
		t.Fatalf("no element carries %s\n%s", attribute, markup)
	}
	open := strings.LastIndex(markup[:at], "<")
	end := strings.Index(markup[at:], ">")
	if open < 0 || end < 0 {
		t.Fatalf("%s is not inside a tag\n%s", attribute, markup)
	}
	return markup[open : at+end+1]
}
func TestTheExperienceFieldHoldsACharacterPastTwentiethLevel(t *testing.T) {
	if CharacterXPLimit <= levelTwentyXP {
		t.Fatalf("the experience limit is %d, which a twentieth level character has already passed", CharacterXPLimit)
	}
	tag := fieldTag(t, characterSheet(t), `id="xp"`)
	if !strings.Contains(tag, `max="`+strconv.Itoa(CharacterXPLimit)+`"`) {
		t.Errorf("the experience field stops somewhere other than its limit\n%s", tag)
	}
	if strings.Contains(tag, `max="9999"`) {
		t.Errorf("the experience field still stops at 9999, which is level four\n%s", tag)
	}
}
func TestEveryNumberFieldStopsWhereTheSaveDoes(t *testing.T) {
	markup := characterSheet(t)
	for _, field := range []struct {
		id    string
		limit int
	}{
		{"xp", CharacterXPLimit},
		{"ac", ACLimit},
		{"max_hp", HPLimit},
		{"current_hp", HPLimit},
		{"temp_hp", HPLimit},
		{"str", AbilityScoreLimit},
	} {
		want := `max="` + strconv.Itoa(field.limit) + `"`
		tag := fieldTag(t, markup, `id="`+field.id+`"`)
		if !strings.Contains(tag, want) {
			t.Errorf("%s does not stop at %d, so a value the save accepts cannot be typed\n%s", field.id, field.limit, tag)
		}
		if !strings.Contains(markup, "Enter a number from 0 to "+strconv.Itoa(field.limit)+".") {
			t.Errorf("%s says nothing about where it stops\n%s", field.id, markup)
		}
	}
}
func TestAPawnOwnedFieldRemembersWhatItShowedInsideItsForm(t *testing.T) {
	data := EditCharacterPageData{CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Size: "small", AC: "17", MaxHP: "140", CurrentHP: "92", TempHP: "0"}
	markup := renderString(t, EditCharacter(data))
	for _, field := range []struct {
		name  string
		panel string
		value string
	}{
		{"size", "identity", "small"},
		{"ac", "core-stats", "17"},
		{"max_hp", "vitals", "140"},
		{"current_hp", "vitals", "92"},
	} {
		tag := fieldTag(t, markup, `id="seen-`+field.name+`"`)
		for _, want := range []string{`type="hidden"`, `name="` + SeenName(field.name) + `"`, `value="` + field.value + `"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("%s: the seen value lacks %s\n%s", field.name, want, tag)
			}
		}
		if strings.Contains(tag, "hx-swap-oob") {
			t.Errorf("%s: the seen value on the page tries to swap itself somewhere\n%s", field.name, tag)
		}
		form := strings.Index(markup, `id="`+panelFormID(field.panel)+`"`)
		at := strings.Index(markup, `id="seen-`+field.name+`"`)
		if form < 0 || at < form || at > form+strings.Index(markup[form:], "</form>") {
			t.Errorf("%s: the seen value is not inside the %s form, so the save never hears it", field.name, field.panel)
		}
	}
}
func TestEverySheetPageListensForTheRoomTab(t *testing.T) {
	data := EditCharacterPageData{CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Derived: testDerivedValues()}
	for _, section := range sheetSections() {
		if !strings.Contains(renderString(t, EditCharacterSection(data, section)), SheetFollowScript) {
			t.Errorf("the %s page never hears the room tab, so it cannot follow a room joined after it opened", section)
		}
	}
}
func TestALivePanelAnswersOnlyItsOwnCharacter(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	trigger := characterPanelTrigger(id)
	if !strings.Contains(trigger, "detail.id === '"+id+"'") {
		t.Errorf("the trigger does not check whose character changed: %s", trigger)
	}
	if !strings.Contains(trigger, typingInPanel) {
		t.Errorf("the trigger would swap a panel out from under the caret: %s", trigger)
	}
}
