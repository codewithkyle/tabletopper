package pages

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

const levelTwentyXP = 355_000

func characterSheet(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	data := EditCharacterPageData{
		XP:        "355000",
		AC:        "17",
		MaxHP:     "140",
		CurrentHP: "92",
		TempHP:    "0",
		Str:       "16",
	}
	if err := EditCharacter(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
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
