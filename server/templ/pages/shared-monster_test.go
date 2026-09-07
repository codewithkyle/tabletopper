package pages

import (
	"strings"
	"testing"
)

// The shared monster page, which is the editor's stat block with the app taken
// away from around it: no bar, no panels, no upload control, and one button that
// is not there for everybody.

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

// The same monster as a signed-out reader sees it, which is the state that puts
// the one link to the app on the page.
func testSharedMonsterForGuest() SharedMonsterData {
	data := testSharedMonster()
	data.Actions.SignIn = "/sign-in"
	data.Actions.Blurb = "Sign in to add this monster to your own manual."

	return data
}

// The whole monster, which is the scope decision this page is built on: the
// import hands over a copy of every column anyway, so a page showing less than
// the button gives would be describing the wrong thing.
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

// A monster whose owner never wrote a paragraph renders no panel for one, the
// way an absent line renders no heading in the block above it. An empty panel
// titled Description reads as a page that failed to load something.
func TestAMonsterWithNoDescriptionRendersNoPanelForOne(t *testing.T) {
	data := testSharedMonster()
	data.Description = ""

	if body := renderToString(t, SharedMonsterPage(data)); strings.Contains(body, "Description") {
		t.Errorf("an empty description rendered a panel anyway:\n%s", body)
	}
}

// THE PICTURE COMES FROM THE SHARE AND NEVER FROM /assets/images. That route
// needs a session, so on this page it would be a broken image for every reader
// who is not signed in -- which is most of them, and the owner testing the link
// would be the one person who could not see it happening.
func TestASharedMonstersPictureNeverNamesTheAppsOwnImageRoute(t *testing.T) {
	body := renderToString(t, SharedMonsterPage(testSharedMonster()))

	if !strings.Contains(body, `src="/share/tok/portrait"`) {
		t.Errorf("the picture is not the share's own:\n%s", body)
	}
	if strings.Contains(body, "/assets/images/") {
		t.Errorf("the page reaches for an image route that needs a session:\n%s", body)
	}
}

// The actions row, in each of the three states a reader can be in. The owner's
// is the one worth pinning: their row is the export alone, because importing
// their own monster would hand them a duplicate they did not ask for.
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

// A monster shared before it was named still has to render a heading and a tab
// title, and a blank line where either goes reads as a page that failed.
func TestAnUnnamedSharedMonsterStillHasATitle(t *testing.T) {
	if got := SharedMonsterTitle("   "); !strings.HasPrefix(got, "Unnamed monster") {
		t.Errorf("SharedMonsterTitle(blank) = %q", got)
	}

	body := renderToString(t, SharedMonsterPage(SharedMonsterData{}))
	if !strings.Contains(body, "Unnamed monster") {
		t.Errorf("an unnamed monster rendered no heading:\n%s", body)
	}
}
