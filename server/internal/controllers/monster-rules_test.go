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

// Every statement names the owner. A handler cannot notice a query that forgets
// one -- the rows would arrive and render -- so the statements themselves are
// checked, which is also the only way to reach the reads: they go through
// QueryContext and the fake pool cannot serve one.
func TestEveryMonsterQueryIsScopedToTheOwner(t *testing.T) {
	for name, body := range namedStatements(t, "monsters.sql") {
		if !strings.Contains(body, "owner_id") {
			t.Errorf("%s is not scoped to the owner:\n%s", name, body)
		}
	}
}

// The action rows are reached through two ids, both of which arrive in the URL
// and neither of which is trusted. The insert names the monster in its own
// WHERE, which is what stops a row being hung off a stranger's stat block, so it
// is covered by the same rule as the rest.
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

// namedStatements splits one query file into the statements sqlc names, so a
// test can read each on its own.
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

// THE THIRTY-FOUR RATINGS ARE THE WHOLE OF THE XP AND PROFICIENCY ARITHMETIC, so
// the table is what is checked rather than the function reading it. A wrong
// number here is a monster worth the wrong experience and a saving throw off by
// one, and neither would fail anywhere.
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

		// Strictly increasing, because the in-lair figure is the next entry's XP
		// and a rating that was not harder than the one below it would make that
		// sentence a lie.
		if i > 0 && rating.XP <= ratings[i-1].XP {
			t.Errorf("CR %s is worth %d XP, which is not more than CR %s at %d",
				rating.Value, rating.XP, ratings[i-1].Value, ratings[i-1].XP)
		}

		// +2 up to CR 4, then one more every four ratings, to +9 at CR 29. The
		// three fractions sit inside the first band.
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

// testMonster is a goblin-boss-shaped stat block: CR 1 (PB +2), Dex 14 (+2),
// Wisdom 8 (-1), proficient in Stealth, and nothing else filled in.
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

// The two numbers a GM never types: the proficiency bonus every save and skill
// is built on, and the experience the party is paid for winning. Both follow
// from the rating, so changing the rating has to move both -- which is the
// whole reason the xp column went.
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

		// The one rating with two answers in the rules. Nothing stores which,
		// so having an action is the question the block asks instead.
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

		// The in-lair figure is the next rating's XP, and it is printed only for
		// a monster that actually has a lair.
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

		// A rating written around the validator reads as the weakest thing in
		// the book rather than as a monster with no numbers at all.
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

// The proficiency bonus reaches the grids as well as the CR line, which is the
// half of the derivation a spot check on one number would miss: a monster's
// saves and skills move when its rating does.
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

// THE BLOCK PRINTS WHAT A MONSTER HAS AND NOTHING ELSE. A printed stat block
// carries no empty headings, so a monster that resists nothing has no
// Resistances line -- and the two that are always there are always there for a
// reason: Senses ends in a passive score every monster has, and a creature that
// cannot speak says so.
func TestTheStatBlockOmitsWhatTheMonsterHasNot(t *testing.T) {
	// A monster as CreateMonsterFromName leaves it: a name, and the schema's
	// defaults for everything else. The six 10s are among them, which is why
	// they are written out rather than left as the zero value -- a score of 0
	// is a -5 modifier and no row in this table has ever held one.
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

	// Only the rows with a state or a bonus are on the Skills line. All
	// eighteen at their ability modifier would be the ability table again,
	// eighteen rows wide.
	if got := statBlockLine(t, block, "Skills"); got != "Stealth +4" {
		t.Errorf("skills = %q, want the one row that has a proficiency", got)
	}
	if got := statBlockLine(t, block, "Senses"); got != "Darkvision 60 ft., Passive Perception 9" {
		t.Errorf("senses = %q, want the special senses and then the passive score", got)
	}
	if got := statBlockLine(t, block, "CR"); got != "1 (XP 200; PB +2)" {
		t.Errorf("CR line = %q", got)
	}

	// The 2024 block prints the initiative modifier with its passive score in
	// brackets, and the ability table carries the saves that used to be a line.
	if block.Initiative != "+2 (12)" {
		t.Errorf("initiative = %q, want +2 (12)", block.Initiative)
	}
	if len(block.Abilities) != 6 || block.Abilities[1].Label != "DEX" || block.Abilities[1].Save != "+2" {
		t.Errorf("the ability table is wrong: %+v", block.Abilities)
	}
}

// A section is on the block when it has rows, and it opens with the sentence the
// book opens it with. The legendary count is the one part of one of those
// sentences that comes out of a column.
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

	// Actions come before Legendary Actions, which is the ENUM's order and the
	// book's.
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

	// A monster with no count has legendary actions it cannot take, so the
	// sentence loses the count rather than printing a zero.
	monster.LegendaryActionUses = 0
	block = monsterStatBlock(monster, actions, monsterDerived(monster, actions))
	if strings.Contains(block.Sections[1].Intro, "Uses") {
		t.Errorf("a zero count was printed: %q", block.Sections[1].Intro)
	}
}

// UNALIGNED IS PRINTED ON A STAT BLOCK AND HIDDEN ON A CHARACTER, and the
// difference is who chose it: a character's alignment starts NULL and the
// editor falls back to unaligned, so it is the answer nobody gave; a beast is
// unaligned because the book says so.
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

// The rest of the line: size and type as one phrase, the tags in brackets after
// them, and the alignment as a clause of its own.
func TestTheSubtitleIsTheLineTheBookPrints(t *testing.T) {
	if got := monsterSubtitle(testMonster()); got != "Small Humanoid (Goblinoid), Chaotic Neutral" {
		t.Errorf("subtitle = %q", got)
	}

	// Every monster is some size and some kind of thing, so a row written
	// around the selects falls back to the column's own defaults rather than
	// printing a bare alignment.
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
