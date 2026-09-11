package controllers
import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"tabletopper/internal/queries"
	"tabletopper/templ/pages"
)
func TestEveryMonsterQueryIsScopedToTheOwner(t *testing.T) {
	for name, body := range namedStatements(t, "monsters.sql") {
		if !strings.Contains(body, "owner_id") {
			t.Errorf("%s is not scoped to the owner:\n%s", name, body)
		}
	}
}
func TestEveryMonsterActionQueryIsScopedToTheOwnerAndMonster(t *testing.T) {
	for name, body := range namedStatements(t, "monster-actions.sql") {
		if !strings.Contains(body, "owner_id") {
			t.Errorf("%s is not scoped to the owner:\n%s", name, body)
		}
		if !strings.Contains(body, "monster_id") && !strings.Contains(body, "monsters") {
			t.Errorf("%s is not scoped to the monster:\n%s", name, body)
		}
	}
}
func namedStatements(t *testing.T, file string) map[string]string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", file))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}
	found := regexp.MustCompile(`(?m)^-- name: (\w+)`).FindAllStringSubmatchIndex(string(source), -1)
	if len(found) == 0 {
		t.Fatalf("no named queries in sql/%s", file)
	}
	statements := map[string]string{}
	for i, at := range found {
		end := len(source)
		if i+1 < len(found) {
			end = found[i+1][0]
		}
		statements[string(source[at[2]:at[3]])] = string(source[at[0]:end])
	}
	return statements
}
func TestEveryChallengeRatingHasItsNumbers(t *testing.T) {
	ratings := pages.ChallengeRatings()
	if len(ratings) != 34 {
		t.Fatalf("%d ratings, want 34 (0, 1/8, 1/4, 1/2 and 1 through 30)", len(ratings))
	}
	if ratings[0].Value != "0" || ratings[1].Value != "1/8" || ratings[2].Value != "1/4" || ratings[3].Value != "1/2" {
		t.Errorf("the list does not open with the four ratings below CR 1: %v", ratings[:4])
	}
	if last := ratings[len(ratings)-1]; last.Value != "30" {
		t.Errorf("the list ends at %q, want 30", last.Value)
	}
	for i, rating := range ratings {
		if rating.Label == "" {
			t.Errorf("rating %q has no label", rating.Value)
		}
		if i > 0 && rating.XP <= ratings[i-1].XP {
			t.Errorf("CR %s is worth %d XP, which is not more than CR %s at %d",
				rating.Value, rating.XP, ratings[i-1].Value, ratings[i-1].XP)
		}
		want := uint8(2)
		if i >= 4 {
			want = uint8(2 + (i-4)/4)
		}
		if rating.Proficiency != want {
			t.Errorf("CR %s has a proficiency bonus of +%d, want +%d", rating.Value, rating.Proficiency, want)
		}
	}
	if got := ratings[len(ratings)-1].Proficiency; got != 9 {
		t.Errorf("CR 30 has a proficiency bonus of +%d, want +9", got)
	}
}
func testMonster() queries.Monster {
	return queries.Monster{
		Name:                     "Goblin Boss",
		Size:                     "small",
		Type:                     "humanoid",
		Tags:                     "Goblinoid",
		Alignment:                "chaotic neutral",
		AC:                       17,
		HP:                       21,
		HitDice:                  "6d6",
		Speed:                    "30 ft.",
		CR:                       "1",
		Str:                      10,
		Dex:                      14,
		Con:                      10,
		Int:                      10,
		Wis:                      8,
		Cha:                      10,
		Skills:                   json.RawMessage(`{}`),
		SkillProficiencies:       json.RawMessage(`{"stealth": "proficient"}`),
		SavingThrows:             json.RawMessage(`{}`),
		SavingThrowProficiencies: json.RawMessage(`{}`),
		Senses:                   "Darkvision 60 ft.",
		Languages:                "Common, Goblin",
	}
}
func monsterActionRow(kind string) queries.MonsterAction {
	return queries.MonsterAction{
		Kind:        queries.MonsterActionsKind(kind),
		Name:        "Bite",
		Description: "Melee Attack Roll: +4, reach 5 ft. Hit: 5 (1d6 + 2) piercing damage.",
	}
}
func TestCombatDerivesProficiencyAndXPFromCR(t *testing.T) {
	for _, c := range []struct {
		name        string
		cr          string
		actions     []queries.MonsterAction
		proficiency string
		xp          string
		inLair      string
	}{
		{name: "a rating in the middle", cr: "5", proficiency: "+3", xp: "1,800"},
		{name: "the top of the table", cr: "30", proficiency: "+9", xp: "155,000"},
		{name: "a fraction", cr: "1/4", proficiency: "+2", xp: "50"},
		{name: "CR 0 with nothing to hit with", cr: "0", proficiency: "+2", xp: "0"},
		{
			name:        "CR 0 that can hurt somebody",
			cr:          "0",
			actions:     []queries.MonsterAction{monsterActionRow(pages.MonsterActionKindAction)},
			proficiency: "+2",
			xp:          "10",
		},
		{
			name:        "a trait is not an action",
			cr:          "0",
			actions:     []queries.MonsterAction{monsterActionRow(pages.MonsterActionKindTrait)},
			proficiency: "+2",
			xp:          "0",
		},
		{name: "no lair, no second figure", cr: "17", proficiency: "+6", xp: "18,000"},
		{
			name:        "a lair action buys the rating above",
			cr:          "17",
			actions:     []queries.MonsterAction{monsterActionRow(pages.MonsterActionKindLairAction)},
			proficiency: "+6",
			xp:          "18,000",
			inLair:      "20,000",
		},
		{
			name:        "CR 30 has nothing above it to be harder than",
			cr:          "30",
			actions:     []queries.MonsterAction{monsterActionRow(pages.MonsterActionKindLairAction)},
			proficiency: "+9",
			xp:          "155,000",
		},
		{name: "a rating nothing wrote", cr: "31", proficiency: "+2", xp: "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			monster := testMonster()
			monster.CR = c.cr
			derived := monsterDerived(monster, c.actions)
			if derived.Proficiency != c.proficiency {
				t.Errorf("proficiency = %s, want %s", derived.Proficiency, c.proficiency)
			}
			if derived.XP != c.xp {
				t.Errorf("XP = %s, want %s", derived.XP, c.xp)
			}
			if derived.InLairXP != c.inLair {
				t.Errorf("in-lair XP = %q, want %q", derived.InLairXP, c.inLair)
			}
		})
	}
}
func TestARatingChangeMovesEverySaveAndSkill(t *testing.T) {
	monster := testMonster()
	monster.SavingThrowProficiencies = json.RawMessage(`{"dex": "proficient"}`)
	if got := bonusRowTotal(monsterDerived(monster, nil).SavingThrows, "dex"); got != "+4" {
		t.Errorf("dex save at CR 1 = %s, want +4 (dex +2, proficient +2)", got)
	}
	if got := bonusRowTotal(monsterDerived(monster, nil).Skills, "stealth"); got != "+4" {
		t.Errorf("stealth at CR 1 = %s, want +4 (dex +2, proficient +2)", got)
	}
	monster.CR = "17"
	if got := bonusRowTotal(monsterDerived(monster, nil).SavingThrows, "dex"); got != "+8" {
		t.Errorf("dex save at CR 17 = %s, want +8 (dex +2, proficient +6)", got)
	}
}
func TestTheStatBlockOmitsWhatTheMonsterHasNot(t *testing.T) {
	bare := queries.Monster{
		Name: "Nothing In Particular",
		CR:   pages.DefaultChallengeRating,
		Str:  10, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10,
	}
	block := monsterStatBlock(bare, nil, monsterDerived(bare, nil))
	if labels := statBlockLabels(block); strings.Join(labels, ",") != "Senses,Languages,CR" {
		t.Errorf("a bare monster prints %v, want only Senses, Languages and CR", labels)
	}
	if got := statBlockLine(t, block, "Languages"); got != "None" {
		t.Errorf("languages = %q, want None", got)
	}
	if got := statBlockLine(t, block, "Senses"); got != "Passive Perception 10" {
		t.Errorf("senses = %q, want the passive score on its own", got)
	}
	if len(block.Sections) != 0 {
		t.Errorf("a monster with no rows has %d sections", len(block.Sections))
	}
	monster := testMonster()
	monster.Vulnerabilities = "Fire"
	monster.Immunities = "Poison; Poisoned"
	monster.Gear = "Chain Shirt, Shortsword"
	block = monsterStatBlock(monster, nil, monsterDerived(monster, nil))
	if labels := statBlockLabels(block); strings.Join(labels, ",") != "Skills,Vulnerabilities,Immunities,Gear,Senses,Languages,CR" {
		t.Errorf("lines = %v; Resistances is the one it has not got", labels)
	}
	if got := statBlockLine(t, block, "Skills"); got != "Stealth +4" {
		t.Errorf("skills = %q, want the one row that has a proficiency", got)
	}
	if got := statBlockLine(t, block, "Senses"); got != "Darkvision 60 ft., Passive Perception 9" {
		t.Errorf("senses = %q, want the special senses and then the passive score", got)
	}
	if got := statBlockLine(t, block, "CR"); got != "1 (XP 200; PB +2)" {
		t.Errorf("CR line = %q", got)
	}
	if block.Initiative != "+2 (12)" {
		t.Errorf("initiative = %q, want +2 (12)", block.Initiative)
	}
	if len(block.Abilities) != 6 || block.Abilities[1].Label != "DEX" || block.Abilities[1].Save != "+2" {
		t.Errorf("the ability table is wrong: %+v", block.Abilities)
	}
}
func TestASectionCarriesTheSentenceTheBookOpensItWith(t *testing.T) {
	monster := testMonster()
	monster.LegendaryActionUses = 3
	actions := []queries.MonsterAction{
		monsterActionRow(pages.MonsterActionKindLegendaryAction),
		monsterActionRow(pages.MonsterActionKindAction),
	}
	block := monsterStatBlock(monster, actions, monsterDerived(monster, actions))
	if len(block.Sections) != 2 {
		t.Fatalf("%d sections, want Actions and Legendary Actions", len(block.Sections))
	}
	if block.Sections[0].Heading != "Actions" || block.Sections[1].Heading != "Legendary Actions" {
		t.Errorf("sections are out of order: %q then %q", block.Sections[0].Heading, block.Sections[1].Heading)
	}
	if block.Sections[0].Intro != "" {
		t.Errorf("the Actions section grew an opening sentence: %q", block.Sections[0].Intro)
	}
	legendary := block.Sections[1].Intro
	if !strings.HasPrefix(legendary, "Legendary Action Uses: 3. ") {
		t.Errorf("legendary intro = %q, want the uses count in front of the sentence", legendary)
	}
	if strings.Contains(legendary, "in Lair") {
		t.Errorf("a monster whose count does not change in its lair printed the parenthetical: %q", legendary)
	}
	monster.LegendaryActionUsesInLair = 4
	block = monsterStatBlock(monster, actions, monsterDerived(monster, actions))
	if !strings.HasPrefix(block.Sections[1].Intro, "Legendary Action Uses: 3 (4 in Lair). ") {
		t.Errorf("legendary intro = %q, want the in-lair count in brackets", block.Sections[1].Intro)
	}
	monster.LegendaryActionUses = 0
	block = monsterStatBlock(monster, actions, monsterDerived(monster, actions))
	if strings.Contains(block.Sections[1].Intro, "Uses") {
		t.Errorf("a zero count was printed: %q", block.Sections[1].Intro)
	}
}
func TestTheSubtitlePrintsUnaligned(t *testing.T) {
	monster := testMonster()
	monster.Alignment = "unaligned"
	monster.Tags = ""
	if got := monsterSubtitle(monster); got != "Small Humanoid, Unaligned" {
		t.Errorf("subtitle = %q, want the alignment printed", got)
	}
	character := queries.Character{Alignment: sql.NullString{String: "unaligned", Valid: true}}
	if got := characterSubtitle(character); strings.Contains(strings.ToLower(got), "unaligned") {
		t.Errorf("the character subtitle printed the alignment nobody chose: %q", got)
	}
}
func TestTheSubtitleIsTheLineTheBookPrints(t *testing.T) {
	if got := monsterSubtitle(testMonster()); got != "Small Humanoid (Goblinoid), Chaotic Neutral" {
		t.Errorf("subtitle = %q", got)
	}
	unwritten := queries.Monster{Size: "enormous", Type: "wyrm", Alignment: "sideways"}
	if got := monsterSubtitle(unwritten); got != "Medium Humanoid" {
		t.Errorf("subtitle = %q, want the defaults and no alignment", got)
	}
}
func statBlockLabels(block pages.StatBlock) []string {
	labels := make([]string, 0, len(block.Lines))
	for _, line := range block.Lines {
		labels = append(labels, line.Label)
	}
	return labels
}
func statBlockLine(t *testing.T, block pages.StatBlock, label string) string {
	t.Helper()
	for _, line := range block.Lines {
		if line.Label == label {
			return line.Value
		}
	}
	t.Fatalf("no %s line on the block", label)
	return ""
}
