package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
)










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



func testMonsterCard() MonsterSummary {
	return MonsterSummary{
		ID:       "01BX5ZZKBKACTAV9WEVGEMMVS4",
		Name:     "Goblin Boss",
		Subtitle: "Small Humanoid (Goblinoid), Chaotic Neutral",
		CR:       "1",
		AC:       "17",
		HP:       "21",
	}
}

func sectionKinds(sections []MonsterActionSection) []string {
	kinds := make([]string, 0, len(sections))
	for _, section := range sections {
		kinds = append(kinds, section.Kind)
	}

	return kinds
}



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

	
	if strings.ReplaceAll(outOfBand, ` hx-swap-oob="true"`, "") != inPage {
		t.Error("the two renders differ by more than the out-of-band flag")
	}

	
	
	body := markup(t, statBlockBody(testStatBlock()))
	fragment := markup(t, MonsterStatBlockFragment(testStatBlock()))
	if !strings.Contains(inPage, body) {
		t.Error("the editor's panel renders its own copy of the block")
	}
	if !strings.Contains(fragment, body) {
		t.Error("the dialog renders its own copy of the block")
	}

	
	
	
	if strings.Contains(fragment, "shadow-panel") {
		t.Error("the dialog draws a panel inside .modal-box, which is already one")
	}
	if strings.Contains(fragment, `id="stat-block"`) {
		t.Error("the dialog carries the editor's swap target, so a redraw could land in it")
	}

	
	
	
	
	
	panel := markup(t, MonsterStatBlockPanel(testStatBlock()))
	if !strings.Contains(panel, body) {
		t.Error("the window renders its own copy of the block")
	}
	if strings.Contains(panel, "modal:close") || strings.Contains(panel, ">Close<") {
		t.Errorf("the window's block carries a Close that closes nothing:\n%s", panel)
	}
	if strings.Contains(panel, "shadow-panel") || strings.Contains(panel, `id="stat-block"`) {
		t.Error("the window's block brings a surface or a swap target of its own")
	}

	if !strings.Contains(fragment, "modal:close") || !strings.Contains(fragment, ">Close<") {
		t.Errorf("the dialog has no way out of it:\n%s", fragment)
	}
}




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



func TestTheMonsterEditorIsOneFormPerPanel(t *testing.T) {
	id := "01BX5ZZKBKACTAV9WEVGEMMVS4"
	page := markup(t, EditMonster(EditMonsterPageData{MonsterID: id, StatBlock: testStatBlock()}))

	for _, panel := range []string{"identity", "abilities", "combat", "defenses", "description", "bonuses/skills", "bonuses/saving_throws"} {
		action := `hx-post="/monsters/` + id + `/` + panel + `"`
		if !strings.Contains(page, action) {
			t.Errorf("no panel posts to %s", action)
		}
	}

	
	if forms := strings.Count(page, "<form"); forms != 7+closingForms {
		t.Errorf("the editor renders %d forms, want %d panels plus the %d the layout carries", forms, 7, closingForms)
	}
}







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
		
		
		if section.Intro != "" && !strings.Contains(page, templ.EscapeString(section.Intro)) {
			t.Errorf("%q drops the sentence the book opens it with", section.Kind)
		}
	}
}



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









func TestTheManualSearchBoxIsInTheBar(t *testing.T) {
	page := renderToString(t, Monsters(MonsterListData{Monsters: []MonsterSummary{testMonsterCard()}}))

	bar := strings.Index(page, "</header>")
	if bar < 0 {
		t.Fatal("the manual has no app bar")
	}
	box := strings.Index(page, `name="q"`)
	if box < 0 {
		t.Fatal("the manual has no search box")
	}
	if box > bar {
		t.Error("the search box is below the bar, on the grid paper with the cards")
	}

	
	
	if !strings.Contains(page, `hx-target="#`+monsterCardsID+`"`) {
		t.Errorf("the search box does not target the card grid")
	}
	if !strings.Contains(page, `id="`+monsterCardsID+`"`) {
		t.Errorf("nothing on the page carries the id the search box swaps")
	}
}





func TestTheManualAndTheEditorCarryTheSameImageControl(t *testing.T) {
	const id = "01BX5ZZKBKACTAV9WEVGEMMVS4"

	card := renderToString(t, MonsterCard(testMonsterCard()))
	editor := renderToString(t, EditMonster(EditMonsterPageData{
		MonsterID: id,
		Header:    MonsterHeader{MonsterID: id, Name: "Goblin Boss"},
	}))

	for _, want := range []string{
		`hx-post="/monsters/` + id + `/image"`,
		`hx-target="closest monster-image"`,
		`hx-swap="outerHTML"`,
		`name="image"`,
	} {
		if !strings.Contains(card, want) {
			t.Errorf("the manual card is missing %s", want)
		}
		if !strings.Contains(editor, want) {
			t.Errorf("the editor's bar is missing %s", want)
		}
	}

	
	reply := renderToString(t, MonsterImageControl(MonsterImage{MonsterID: id, Name: "Goblin Boss"}))
	if !strings.Contains(card, reply) {
		t.Error("the card does not render the component the upload replies with")
	}
	if !strings.Contains(editor, reply) {
		t.Error("the editor does not render the component the upload replies with")
	}
}




func TestTheNewMonsterDialogCarriesAPicture(t *testing.T) {
	dialog := renderToString(t, NewMonsterFragment())

	if !strings.Contains(dialog, `hx-encoding="multipart/form-data"`) {
		t.Error("the form is not multipart, so a chosen file would never be sent")
	}
	if !strings.Contains(dialog, `type="file"`) || !strings.Contains(dialog, `name="image"`) {
		t.Errorf("the dialog has no picture field:\n%s", dialog)
	}
	
	
	
	picker := regexp.MustCompile(`<input[^>]*type="file"[^>]*>`).FindString(dialog)
	if picker == "" {
		t.Fatalf("no file input in the dialog:\n%s", dialog)
	}
	if strings.Contains(picker, "required") {
		t.Errorf("the picture is required, so a monster cannot be created without one: %s", picker)
	}

	
	
	
	if strings.Index(dialog, `name="name"`) > strings.Index(dialog, `name="image"`) {
		t.Error("the picture field is above the name, so the dialog opens with focus on it")
	}
}




func TestTheImageControlDoesNotCarryItsOwnSize(t *testing.T) {
	control := renderToString(t, MonsterImageControl(MonsterImage{MonsterID: "01BX5ZZKBKACTAV9WEVGEMMVS4", Name: "Goblin Boss"}))

	if !strings.Contains(control, "w-full") {
		t.Error("the control does not fill the box that sizes it")
	}
	
	
	for _, size := range []string{"w-14", "w-11"} {
		if strings.Contains(control, size) {
			t.Errorf("the control carries %s; the page that draws it should", size)
		}
	}
}





func TestTheManualListsOneMonsterPerRow(t *testing.T) {
	page := renderToString(t, Monsters(MonsterListData{
		Monsters: []MonsterSummary{testMonsterCard(), testMonsterCard()},
	}))

	container := regexp.MustCompile(`<section id="` + monsterCardsID + `"[^>]*><div class="([^"]*)"`).FindStringSubmatch(page)
	if container == nil {
		t.Fatal("the card grid is not the first thing in the list section any more")
	}
	if strings.Contains(container[1], "grid-cols") {
		t.Errorf("the cards are laid out in columns: %q", container[1])
	}
}
