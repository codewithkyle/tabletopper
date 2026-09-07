package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// THE SECTION LIST AND THE ENUM ARE ONE LIST WRITTEN TWICE, and this is what
// holds them together. A member added to monster_actions.kind with no section
// here is a row that can be inserted, read back and never rendered -- nothing
// would fail, the stat block would simply be missing a heading.
//
// THE ORDER MATTERS AS MUCH AS THE MEMBERSHIP. Rows come back ORDER BY kind, and
// MySQL sorts an ENUM by the order its members were declared, so the ENUM's
// order IS the stat block's order. A section list that disagreed with it would
// draw the editor's panels in one order and the block's sections in another.
func TestEveryActionKindHasASection(t *testing.T) {
	members := actionKindEnumMembers(t)
	sections := MonsterActionSections()

	if len(sections) != len(members) {
		t.Fatalf("%d sections for %d ENUM members: %v against %v", len(sections), len(members), sectionKinds(sections), members)
	}

	for i, member := range members {
		if sections[i].Kind != member {
			t.Errorf("section %d is %q, want %q -- the list is out of step with the ENUM", i, sections[i].Kind, member)
		}
		if sections[i].Heading == "" {
			t.Errorf("%q has no heading", member)
		}
		if sections[i].Singular == "" {
			t.Errorf("%q has no name for one of its rows", member)
		}
		if sections[i].AddLabel() != "Add "+sections[i].Singular {
			t.Errorf("%q builds its add label from something other than its own noun: %q", member, sections[i].AddLabel())
		}

		if _, ok := MonsterActionSectionFor(member); !ok {
			t.Errorf("%q is a kind the allowlist would refuse", member)
		}
	}

	if _, ok := MonsterActionSectionFor("mythic_action"); ok {
		t.Error("the allowlist admitted a kind that is not in the ENUM")
	}
}

// Three of the seven open with a standing rule the book prints once above the
// list, and the other four open with nothing. Which four is a fact about the
// rules rather than about this list, so it is pinned rather than derived.
func TestOnlyTheThreeSectionsWithAStandingRuleHaveAnIntro(t *testing.T) {
	withIntro := map[string]bool{
		MonsterActionKindLegendaryAction: true,
		MonsterActionKindLairAction:      true,
		MonsterActionKindRegionalEffect:  true,
	}

	for _, section := range MonsterActionSections() {
		switch {
		case withIntro[section.Kind] && section.Intro == "":
			t.Errorf("%q opens with nothing, want the book's sentence", section.Kind)
		case !withIntro[section.Kind] && section.Intro != "":
			t.Errorf("%q grew an opening sentence: %q", section.Kind, section.Intro)
		}
	}
}

// actionKindEnumMembers reads the ENUM out of the schema dump, in the order it
// was declared.
func actionKindEnumMembers(t *testing.T) []string {
	t.Helper()

	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}

	table := regexp.MustCompile("(?s)CREATE TABLE `monster_actions` \\((.*?)\n\\) ENGINE=").FindStringSubmatch(string(schema))
	if table == nil {
		t.Fatal("no monster_actions table in db/schema.sql")
	}

	column := regexp.MustCompile("(?m)^\\s*`kind` enum\\((.*?)\\)").FindStringSubmatch(table[1])
	if column == nil {
		t.Fatalf("no kind ENUM on monster_actions:\n%s", table[1])
	}

	members := []string{}
	for _, member := range strings.Split(column[1], ",") {
		members = append(members, strings.Trim(strings.TrimSpace(member), "'"))
	}

	return members
}

// testMonsterCard is one filled-in card, for the renders that want a grid with
// something in it rather than an empty state.
func testMonsterCard() MonsterSummary {
	return MonsterSummary{
		ID:       "01BX5ZZKBKACTAV9WEVGEMMVS4",
		Name:     "Goblin Boss",
		Subtitle: "Small Humanoid (Goblinoid), Chaotic Neutral",
		CR:       "1",
		AC:       "17",
		HP:       "21",
		Speed:    "30 ft.",
	}
}

func sectionKinds(sections []MonsterActionSection) []string {
	kinds := make([]string, 0, len(sections))
	for _, section := range sections {
		kinds = append(kinds, section.Kind)
	}

	return kinds
}

// testStatBlock is a filled-in block: every line the markup can omit is present,
// and every section kind that has an opening sentence is represented.
func testStatBlock() StatBlock {
	return StatBlock{
		Name:       "Goblin Boss",
		Subtitle:   "Small Humanoid (Goblinoid), Chaotic Neutral",
		AC:         "17",
		Initiative: "+2 (12)",
		HP:         "21",
		HitDice:    "6d6",
		Speed:      "30 ft.",
		Abilities: []StatBlockAbility{
			{Label: "STR", Score: "10", Mod: "+0", Save: "+0"},
			{Label: "DEX", Score: "14", Mod: "+2", Save: "+2"},
			{Label: "CON", Score: "10", Mod: "+0", Save: "+0"},
			{Label: "INT", Score: "10", Mod: "+0", Save: "+0"},
			{Label: "WIS", Score: "8", Mod: "-1", Save: "-1"},
			{Label: "CHA", Score: "10", Mod: "+0", Save: "+0"},
		},
		Lines: []StatBlockEntry{
			{Label: "Skills", Value: "Stealth +4"},
			{Label: "Senses", Value: "Darkvision 60 ft., Passive Perception 9"},
			{Label: "Languages", Value: "Common, Goblin"},
			{Label: "CR", Value: "1 (XP 200; PB +2)"},
		},
		Sections: []StatBlockSection{
			{
				Heading: "Actions",
				Entries: []StatBlockEntry{{Label: "Scimitar", Value: "Melee Attack Roll: +4, reach 5 ft."}},
			},
			{
				Heading: "Lair Actions",
				Intro:   "On initiative count 20 (losing initiative ties), it takes a lair action.",
				Entries: []StatBlockEntry{{Value: "The floor becomes difficult terrain."}},
			},
		},
		Habitat:  "Forest, Urban",
		Treasure: "Individual",
	}
}

// THE SAME COMPONENT SERVES THE EDITOR, THE DIALOG AND THE REFRESH, which is
// what makes the preview beside the editor trustworthy: what a GM sees while
// typing is byte-for-byte what the dialog shows. The only difference between the
// three renders is the one attribute that makes htmx swap it in place.
func TestTheStatBlockIsOneComponentRenderedThreeWays(t *testing.T) {
	inPage := markup(t, MonsterStatBlock(testStatBlock(), false))
	outOfBand := markup(t, MonsterStatBlock(testStatBlock(), true))

	for _, block := range []string{inPage, outOfBand} {
		if !strings.Contains(block, `id="stat-block"`) {
			t.Errorf("the block has no id for a swap to find:\n%s", block)
		}
	}
	if strings.Contains(inPage, "hx-swap-oob") {
		t.Error("the block rendered inside the page would swap itself away")
	}
	if !strings.Contains(outOfBand, `hx-swap-oob="true"`) {
		t.Error("the refreshed block would land in the panel's error slot instead of in place")
	}

	// Everything but that attribute is the same markup.
	if strings.ReplaceAll(outOfBand, ` hx-swap-oob="true"`, "") != inPage {
		t.Error("the two renders differ by more than the out-of-band flag")
	}

	// The dialog is the same block again, with the Close the modal contract
	// requires -- and no second copy of the markup.
	fragment := markup(t, MonsterStatBlockFragment(testStatBlock()))
	if !strings.Contains(fragment, inPage) {
		t.Error("the dialog renders its own copy of the block rather than the component")
	}
	if !strings.Contains(fragment, "modal:close") || !strings.Contains(fragment, ">Close<") {
		t.Errorf("the dialog has no way out of it:\n%s", fragment)
	}
}

// The block prints what a monster has and nothing else, so an empty one is a
// heading and no lines at all -- and it still renders, because that is what the
// editor draws for a monster created a moment ago.
func TestAnEmptyStatBlockStillRenders(t *testing.T) {
	block := markup(t, MonsterStatBlock(StatBlock{}, false))

	if !strings.Contains(block, "Unnamed monster") {
		t.Errorf("an unnamed monster has no heading:\n%s", block)
	}
	for _, absent := range []string{"Skills", "Senses", "Languages", "Habitat", "Treasure"} {
		if strings.Contains(block, ">"+absent+"<") {
			t.Errorf("the block printed a %s line for a monster that has none", absent)
		}
	}
}

// The editor is one form per panel and the block beside them is not a form.
// Every panel posts to its own route, which is what makes the saves disjoint.
func TestTheMonsterEditorIsOneFormPerPanel(t *testing.T) {
	id := "01BX5ZZKBKACTAV9WEVGEMMVS4"
	page := markup(t, EditMonster(EditMonsterPageData{MonsterID: id, StatBlock: testStatBlock()}))

	for _, panel := range []string{"identity", "abilities", "combat", "defenses", "description", "bonuses/skills", "bonuses/saving_throws"} {
		action := `hx-post="/monsters/` + id + `/` + panel + `"`
		if !strings.Contains(page, action) {
			t.Errorf("no panel posts to %s", action)
		}
	}

	// One <form> per saving panel, plus the dialogs the layout always carries.
	if forms := strings.Count(page, "<form"); forms != 7+closingForms {
		t.Errorf("the editor renders %d forms, want %d panels plus the %d the layout carries", forms, 7, closingForms)
	}
}

// THE EDITOR DRAWS ITS SECTIONS BY RANGING OVER THE LIST, so a section cannot be
// left off the page by somebody forgetting to write its markup -- which is the
// failure that would look like a stat block quietly missing its reactions.
//
// Each one needs three things to work: a container for its rows, an add button
// posting to its own kind, and the heading a GM finds it by.
func TestTheEditorRendersEverySection(t *testing.T) {
	id := "01BX5ZZKBKACTAV9WEVGEMMVS4"
	page := markup(t, EditMonster(EditMonsterPageData{MonsterID: id, StatBlock: testStatBlock()}))

	for _, section := range MonsterActionSections() {
		if !strings.Contains(page, `id="actions-`+section.Kind+`"`) {
			t.Errorf("%q has no container for its rows", section.Kind)
		}
		if !strings.Contains(page, `hx-post="/monsters/`+id+`/actions/`+section.Kind+`"`) {
			t.Errorf("%q has no add button, or its button posts somewhere else", section.Kind)
		}
		if !strings.Contains(page, ">"+section.Heading+"<") {
			t.Errorf("%q renders no heading", section.Kind)
		}
		if !strings.Contains(page, ">"+section.AddLabel()+"<") {
			t.Errorf("%q renders no add label", section.Kind)
		}
		// Through the same escaper the renderer uses: three of the sentences
		// carry an apostrophe, and templ writes those as an entity.
		if section.Intro != "" && !strings.Contains(page, templ.EscapeString(section.Intro)) {
			t.Errorf("%q drops the sentence the book opens it with", section.Kind)
		}
	}
}

// A row is its own form, posting to its own URL on its own debounce -- which is
// what keeps a GM typing a Bite from rewriting the Multiattack above it.
func TestTwoMonsterActionRowsShareNoElementID(t *testing.T) {
	id := "01BX5ZZKBKACTAV9WEVGEMMVS4"
	first := MonsterAction{ID: "01BX5ZZKBKACTAV9WEVGEMMVS5", Kind: MonsterActionKindAction, Name: "Bite"}
	second := MonsterAction{ID: "01BX5ZZKBKACTAV9WEVGEMMVS6", Kind: MonsterActionKindAction, Name: "Claw"}

	page := markup(t, EditMonster(EditMonsterPageData{
		MonsterID: id,
		StatBlock: testStatBlock(),
		Actions:   map[string][]MonsterAction{MonsterActionKindAction: {first, second}},
	}))

	for _, row := range []MonsterAction{first, second} {
		action := "/monsters/" + id + "/actions/" + row.Kind + "/" + row.ID
		if !strings.Contains(page, `hx-post="`+action+`"`) {
			t.Errorf("the %s row does not save to its own URL", row.Name)
		}
		if !strings.Contains(page, `hx-delete="`+action+`"`) {
			t.Errorf("the %s row does not delete itself", row.Name)
		}
		if !strings.Contains(page, `id="errors-`+MonsterActionRowPanel(row.ID)+`"`) {
			t.Errorf("the %s row has no error block of its own", row.Name)
		}
	}
}
