package pages

import (
	"strings"
	"testing"
)

func testSharedMonster() SharedMonsterData {
	return SharedMonsterData{
		Block: StatBlock{
			Name:       "Ancient Red Dragon",
			Subtitle:   "Gargantuan Dragon (Chromatic), Chaotic Evil",
			Image:      "/share/tok/portrait",
			AC:         "22",
			Initiative: "+14 (24)",
			HP:         "507",
			HitDice:    "26d20 + 234",
			Speed:      "40 ft., climb 40 ft., fly 80 ft.",
			Abilities: []StatBlockAbility{
				{Label: "STR", Score: "30", Mod: "+10", Save: "+10"},
				{Label: "DEX", Score: "10", Mod: "+0", Save: "+7"},
			},
			Lines: []StatBlockEntry{{Label: "Immunities", Value: "Fire"}},
			Sections: []StatBlockSection{{
				Heading: "Actions",
				Entries: []StatBlockEntry{{Label: "Bite", Value: "Melee Attack Roll: +17"}},
			}},
			Habitat:  "Mountain, Underdark",
			Treasure: "Horde",
		},
		Description: "It has slept under the peak for four centuries.",
		Actions:     SharedActions{Export: "/share/tok/export.md"},
	}
}
func testSharedMonsterForGuest() SharedMonsterData {
	data := testSharedMonster()
	data.Actions.SignIn = "/sign-in"
	data.Actions.Blurb = "Sign in to add this monster to your own manual."
	return data
}
func TestASharedMonsterRendersTheWholeStatBlock(t *testing.T) {
	body := renderToString(t, SharedMonsterPage(testSharedMonster()))
	for _, want := range []string{
		"Ancient Red Dragon", "Gargantuan Dragon (Chromatic), Chaotic Evil",
		"507", "26d20 + 234", "climb 40 ft.",
		"STR", "+10", "Immunities", "Fire",
		"Actions", "Bite", "Melee Attack Roll: +17",
		"Mountain, Underdark", "Horde",
		"Description", "It has slept under the peak for four centuries.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not show %q:\n%s", want, body)
		}
	}
}
func TestAMonsterWithNoDescriptionRendersNoPanelForOne(t *testing.T) {
	data := testSharedMonster()
	data.Description = ""
	if body := renderToString(t, SharedMonsterPage(data)); strings.Contains(body, "Description") {
		t.Errorf("an empty description rendered a panel anyway:\n%s", body)
	}
}
func TestASharedMonstersPictureNeverNamesTheAppsOwnImageRoute(t *testing.T) {
	body := renderToString(t, SharedMonsterPage(testSharedMonster()))
	if !strings.Contains(body, `src="/share/tok/portrait"`) {
		t.Errorf("the picture is not the share's own:\n%s", body)
	}
	if strings.Contains(body, "/assets/images/") {
		t.Errorf("the page reaches for an image route that needs a session:\n%s", body)
	}
}
func TestTheImportPanelIsDrawnForWhoeverCanUseIt(t *testing.T) {
	data := testSharedMonster()
	export := "/share/tok/export.md"
	data.Actions = SharedActions{Export: export, Import: "/share/tok/import", Blurb: "Add this monster."}
	reader := renderToString(t, SharedMonsterPage(data))
	if !strings.Contains(reader, `action="/share/tok/import"`) {
		t.Errorf("a signed-in reader is not offered the copy:\n%s", reader)
	}
	if !strings.Contains(reader, `method="post"`) {
		t.Errorf("the import is not a plain form post:\n%s", reader)
	}
	if !strings.Contains(reader, "Add to my manual") {
		t.Errorf("the button does not say what it does:\n%s", reader)
	}
	data.Actions = SharedActions{Export: export, SignIn: "/sign-in", Blurb: "Sign in to add this monster."}
	guest := renderToString(t, SharedMonsterPage(data))
	if !strings.Contains(guest, `href="/sign-in"`) {
		t.Errorf("a signed-out reader is not told where the copy comes from:\n%s", guest)
	}
	if strings.Contains(guest, "<form") {
		t.Errorf("a signed-out reader is offered a form that would bounce:\n%s", guest)
	}
	data.Actions = SharedActions{Export: export}
	owner := renderToString(t, SharedMonsterPage(data))
	if strings.Contains(owner, "manual") || strings.Contains(owner, "/sign-in") {
		t.Errorf("the owner is offered an import:\n%s", owner)
	}
}
func TestAnUnnamedSharedMonsterStillHasATitle(t *testing.T) {
	if got := SharedMonsterTitle("   "); !strings.HasPrefix(got, "Unnamed monster") {
		t.Errorf("SharedMonsterTitle(blank) = %q", got)
	}
	body := renderToString(t, SharedMonsterPage(SharedMonsterData{}))
	if !strings.Contains(body, "Unnamed monster") {
		t.Errorf("an unnamed monster rendered no heading:\n%s", body)
	}
}
