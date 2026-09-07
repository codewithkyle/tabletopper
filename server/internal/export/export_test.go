package export

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"tabletopper/templ/pages"
)

// The file is the deliverable here, so most of these read it back rather than
// checking that a function was called. What they are protecting is not prose --
// it is the three things a generated Markdown file gets wrong: a block that
// renders as one run-on paragraph, a property block a parser refuses, and a
// table row that silently shifts every cell after it one column left.

func fullStatBlock() pages.StatBlock {
	return pages.StatBlock{
		Name:       "Ancient Red Dragon",
		Subtitle:   "Gargantuan Dragon (Chromatic), Chaotic Evil",
		Image:      "/share/tok/portrait",
		AC:         "22",
		Initiative: "+14 (24)",
		HP:         "507",
		HitDice:    "26d20 + 234",
		Speed:      "40 ft., fly 80 ft.",
		Abilities: []pages.StatBlockAbility{
			{Label: "STR", Score: "30", Mod: "+10", Save: "+17"},
		},
		Lines: []pages.StatBlockEntry{
			{Label: "Immunities", Value: "Fire"},
			{Label: "CR", Value: "24 (XP 62,000; PB +7)"},
		},
		Sections: []pages.StatBlockSection{
			{
				Heading: "Legendary Actions",
				Intro:   "Legendary Action Uses: 3 (4 in Lair).",
				Entries: []pages.StatBlockEntry{{Label: "Frightful Presence", Value: "It casts Fear."}},
			},
			{
				Heading: "Regional Effects",
				Entries: []pages.StatBlockEntry{{Value: "Small earthquakes are common."}},
			},
		},
		Habitat:  "Mountain",
		Treasure: "Horde",
	}
}

func fullSheet() pages.SharedCharacterSheet {
	return pages.SharedCharacterSheet{
		Header: pages.CharacterHeader{
			Name: "Vex", Subtitle: "Half-Elf | Ranger 5", AvatarID: "01SHOULDNOTAPPEAR0000000AB",
			AC: "16", CurrentHP: "31", MaxHP: "38", Speed: "30 ft.",
			Initiative: "+3", Proficiency: "+3", Passive: "15",
		},
		Avatar:            "/share/tok/portrait",
		Identity:          []pages.SharedFact{{Label: "Species", Value: "Half-Elf"}},
		CoreStats:         []pages.SharedFact{{Label: "Armor Class", Value: "16"}},
		Spellcasting:      []pages.SharedFact{{Label: "Spell Save DC", Value: "14"}},
		Vitals:            []pages.SharedFact{{Label: "Hit Dice", Value: "5d10"}},
		Abilities:         []pages.SharedAbility{{Label: "Dexterity", Score: "18", Mod: "+4"}},
		SavingThrows:      []pages.SharedBonus{{Label: "Dexterity", Total: "+7"}},
		Skills:            []pages.SharedBonus{{Label: "Stealth", Abbr: "DEX", Total: "+9"}},
		PassivePerception: "15",
		Training:          []pages.SharedFact{{Label: "Languages", Value: "Common, Elvish"}},
		Attacks: []pages.SharedAttack{
			{Name: "Longbow", Bonus: "+8", Damage: "1d8+5", DamageType: "Piercing", Mastery: "Slow", Notes: "Silvered."},
		},
		Features:   []pages.SharedFact{{Label: "Favoured Enemy", Value: "Undead"}},
		Equipped:   []pages.SharedItem{{Name: "Arrow", Quantity: "20", Description: "In a quiver."}},
		SpellSlots: []pages.SharedSpellLevel{{Name: "Level 1", Slots: "4 slots", Spells: "3 spells"}},
		Prepared: []pages.SharedSpellGroup{{
			Name:   "Level 1",
			Spells: []pages.SharedSpell{{Name: "Hunter's Mark", Meta: "1 bonus action", Description: "An extra 1d6."}},
		}},
		Personality: []pages.SharedFact{{Label: "Bonds", Value: "My brother."}},
		Appearance:  []pages.SharedFact{{Label: "Eyes", Value: "Grey"}},
	}
}

// EVERY VALUE THE PAGE WAS GIVEN REACHES THE FILE. A section quietly missing
// from an export is the failure mode this whole package has -- nothing errors,
// the file downloads, and a GM feeding it to a model gets an answer built on a
// character with no spells. So both fixtures are filled in and every value is
// looked for by name.
func TestEveryValueOnTheSheetReachesTheFile(t *testing.T) {
	file := string(Character(fullSheet()))

	for _, want := range []string{
		"Vex", "Half-Elf | Ranger 5",
		"## Identity", "Species", "Half-Elf",
		"## Core Stats", "Armor Class", "Spell Save DC", "14",
		"## Vitals", "Hit Dice", "5d10",
		"## Abilities", "Dexterity", "18", "+4",
		"## Saving Throws", "+7",
		"## Skills", "Stealth", "+9", "Passive Perception",
		"## Proficiencies & Training", "Common, Elvish",
		"## Attacks", "Longbow", "1d8+5", "Piercing", "Slow", "Silvered.",
		"## Features & Traits", "Favoured Enemy", "Undead",
		"## Equipment", "Arrow ×20", "In a quiver.",
		"## Spell Slots", "4 slots", "3 spells",
		"## Prepared Spells", "### Level 1", "Hunter's Mark", "1 bonus action", "An extra 1d6.",
		"## Personality", "My brother.",
		"## Appearance", "Grey",
	} {
		if !strings.Contains(file, want) {
			t.Errorf("the exported sheet is missing %q:\n%s", want, file)
		}
	}
}

func TestEveryValueOnTheStatBlockReachesTheFile(t *testing.T) {
	file := string(Monster(fullStatBlock(), "It has slept for four centuries."))

	for _, want := range []string{
		"# Ancient Red Dragon", "*Gargantuan Dragon (Chromatic), Chaotic Evil*",
		"- **AC** 22", "- **Initiative** +14 (24)", "- **HP** 507 (26d20 + 234)", "- **Speed** 40 ft., fly 80 ft.",
		"| STR | 30 | +10 | +17 |",
		"- **Immunities** Fire", "- **CR** 24 (XP 62,000; PB +7)",
		"## Legendary Actions", "*Legendary Action Uses: 3 (4 in Lair).*",
		"**Frightful Presence.** It casts Fear.",
		"## Regional Effects", "Small earthquakes are common.",
		"## Habitat and Treasure", "- **Habitat** Mountain", "- **Treasure** Horde",
		"## Description", "It has slept for four centuries.",
	} {
		if !strings.Contains(file, want) {
			t.Errorf("the exported monster is missing %q:\n%s", want, file)
		}
	}
}

// A REGIONAL EFFECT HAS NO NAME and the book prints those as a bare list, so the
// file has to be able to write a paragraph with nothing bold in front of it. The
// alternative -- an empty pair of asterisks -- renders as two stray characters.
func TestAnUnnamedEntryIsWrittenAsAPlainParagraph(t *testing.T) {
	file := string(Monster(fullStatBlock(), ""))

	if strings.Contains(file, "**.**") || strings.Contains(file, "**** ") {
		t.Errorf("an unnamed entry rendered an empty bold label:\n%s", file)
	}
}

// NO PICTURE AND NO ID EVER REACHES THE FILE. Every image URL in this app needs
// a session or a token, so a reference to one is a broken image in somebody's
// vault forever -- and a file that named the row it came from would be a thing
// pasted into a chat window that says which account it belongs to.
func TestNoPictureAndNoIdentifierIsWrittenIntoTheFile(t *testing.T) {
	for name, file := range map[string]string{
		"monster":   string(Monster(fullStatBlock(), "")),
		"character": string(Character(fullSheet())),
	} {
		t.Run(name, func(t *testing.T) {
			for _, forbidden := range []string{"/share/", "/assets/", "01SHOULDNOTAPPEAR", "![", "<img"} {
				if strings.Contains(file, forbidden) {
					t.Errorf("the file carries %q:\n%s", forbidden, file)
				}
			}
		})
	}
}

// THE PROPERTY BLOCK IS THE ONE PART A PARSER READS STRICTLY, and every value in
// it was typed by a person into a column that allows quotes and newlines. An
// unescaped one ends the value early and turns the rest of the name into broken
// YAML -- which Obsidian reports as a file with no properties at all rather than
// as an error anybody could act on.
func TestAnAwkwardNameCannotBreakTheFrontmatter(t *testing.T) {
	block := fullStatBlock()
	block.Name = "The \"Dragon\"\nof \\ Doom\r\n"

	file := string(Monster(block, ""))
	front, _, found := strings.Cut(strings.TrimPrefix(file, "---\n"), "\n---\n")
	if !found {
		t.Fatalf("the file has no frontmatter block:\n%s", file)
	}

	if strings.Contains(front, "\n---") {
		t.Errorf("the frontmatter ends early:\n%s", front)
	}
	// Every property is `key: "value"`, except the one list -- `tags:` and an
	// indented member under it.
	for _, line := range strings.Split(front, "\n") {
		if line == "tags:" || strings.HasPrefix(line, "  - ") {
			continue
		}
		if !regexp.MustCompile(`^[a-z_]+: ".*"$`).MatchString(line) {
			t.Errorf("frontmatter line is not a quoted string: %q", line)
		}
	}
	if !strings.Contains(front, `\"Dragon\"`) || !strings.Contains(front, `\\`) {
		t.Errorf("the quotes and the backslash were not escaped:\n%s", front)
	}
}

// A COLUMN NOTHING FILLS IS NOT WRITTEN. Abbr is set on every skill and on no
// saving throw -- the ability a row keys off is worth saying under Acrobatics
// and is not under Strength -- so the two tables differ by a column, exactly as
// the markup differs by a span it does not print.
func TestTheSavingThrowsTableHasNoEmptyAbilityColumn(t *testing.T) {
	file := string(Character(fullSheet()))

	saves, _, found := strings.Cut(strings.SplitN(file, "## Saving Throws\n\n", 2)[1], "\n\n")
	if !found {
		t.Fatalf("no saving throws table:\n%s", file)
	}
	if strings.Contains(saves, "Ability") {
		t.Errorf("the saving throws table keeps a column nothing fills:\n%s", saves)
	}
	if !strings.Contains(saves, "| Dexterity | +7 |") {
		t.Errorf("the saving throws table lost its rows:\n%s", saves)
	}

	skills, _, _ := strings.Cut(strings.SplitN(file, "## Skills\n\n", 2)[1], "\n\n")
	if !strings.Contains(skills, "| Skill | Ability | Bonus |") {
		t.Errorf("the skills table dropped the column that is filled:\n%s", skills)
	}
}

// A PIPE IN A CELL OPENS A COLUMN THE DIVIDER ROW HAS NO WIDTH FOR, and every
// cell after it in that row lands under the wrong heading -- so a skill somebody
// called "Sleight of Hand | Cards" would silently move their bonus into the
// ability column. It is the one place the body is escaped.
func TestAPipeInAValueCannotOpenAColumn(t *testing.T) {
	sheet := fullSheet()
	sheet.Skills = []pages.SharedBonus{{Label: "Cards | Dice", Abbr: "DEX", Total: "+9"}}

	file := string(Character(sheet))
	if !strings.Contains(file, `| Cards \| Dice | DEX | +9 |`) {
		t.Errorf("the pipe was not escaped:\n%s", file)
	}

	// Every row of a table has to have the heading row's shape. The file holds
	// several tables of different widths, so each run of rows is measured
	// against its own first line rather than against a number.
	width := 0
	for _, line := range strings.Split(file, "\n") {
		if !strings.HasPrefix(line, "| ") {
			width = 0
			continue
		}

		dividers := strings.Count(line, "|") - strings.Count(line, `\|`)
		if width == 0 {
			width = dividers
			continue
		}
		if dividers != width {
			t.Errorf("table row has %d dividers, want the heading's %d: %q", dividers, width, line)
		}
	}
}

// A NEWLINE INSIDE A LABEL OR A CELL BREAKS THE SHAPE AROUND IT: a bullet
// becomes two, and a table row becomes a row and a paragraph. Prose keeps its
// newlines -- that is what prose is -- so only the values that sit inside a line
// of markup are flattened.
func TestAValueInsideMarkupIsFlattened(t *testing.T) {
	sheet := fullSheet()
	sheet.Identity = []pages.SharedFact{{Label: "Species", Value: "Half-Elf\nand proud"}}
	sheet.Attacks = []pages.SharedAttack{{Name: "Longbow", Damage: "1d8\n+5"}}

	file := string(Character(sheet))
	if !strings.Contains(file, "- **Species** Half-Elf and proud") {
		t.Errorf("a bullet's value kept its newline:\n%s", file)
	}
	if !strings.Contains(file, "| Longbow |  | 1d8 +5 |") {
		t.Errorf("a cell kept its newline:\n%s", file)
	}
}

// TWO LINES WITH ONE NEWLINE BETWEEN THEM ARE ONE PARAGRAPH, which is the thing
// that goes wrong in a generated Markdown file: the source looks right and the
// rendered page runs every stat line together. Blocks are what make it
// unrepresentable, and this is the check that they are still doing it.
func TestBlocksAreSeparatedByExactlyOneBlankLine(t *testing.T) {
	for name, file := range map[string]string{
		"monster":   string(Monster(fullStatBlock(), "It has slept.")),
		"character": string(Character(fullSheet())),
	} {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(file, "\n\n\n") {
				t.Errorf("the file has a run of blank lines:\n%s", file)
			}
			if !strings.HasSuffix(file, "\n") || strings.HasSuffix(file, "\n\n") {
				t.Errorf("the file does not end in exactly one newline: %q", file[len(file)-4:])
			}
			for _, line := range strings.Split(file, "\n") {
				if line != strings.TrimRight(line, " \t") {
					t.Errorf("line has trailing whitespace: %q", line)
				}
			}
		})
	}
}

// AN EMPTY SECTION IS NOT WRITTEN AT ALL, which is the page's own rule: a
// character with no attacks has no Attacks panel, and now no Attacks heading
// either. A file listing what somebody has not got is longer and less useful.
func TestAnEmptySheetWritesNoHeadings(t *testing.T) {
	file := string(Character(pages.SharedCharacterSheet{}))

	if strings.Contains(file, "##") {
		t.Errorf("an empty sheet wrote a heading:\n%s", file)
	}
	if !strings.Contains(file, "# Unnamed character") {
		t.Errorf("an unnamed character has no heading at all:\n%s", file)
	}

	block := string(Monster(pages.StatBlock{}, ""))
	if strings.Contains(block, "##") {
		t.Errorf("an empty monster wrote a heading:\n%s", block)
	}
}

// THE FILENAME IS THE ONE VALUE THAT REACHES A HEADER, so it is a slug and not a
// name: a quote in it would end the Content-Disposition parameter early and
// leave the rest of the monster's name as syntax the browser has to guess at.
func TestTheFilenameIsASlugAndNeverTheNameItself(t *testing.T) {
	for name, want := range map[string]string{
		"Ancient Red Dragon":     "ancient-red-dragon.md",
		"  Kuo-toa Whip (Sea)  ": "kuo-toa-whip-sea.md",
		`He said "no"; then ran`: "he-said-no-then-ran.md",
		"Cœur de Lion":           "c-ur-de-lion.md",
		"...":                    "monster.md",
		"":                       "monster.md",
		"日本語":                    "monster.md",
	} {
		if got := Filename(name, "monster"); got != want {
			t.Errorf("Filename(%q) = %q, want %q", name, got, want)
		}
	}

	long := Filename(strings.Repeat("dragon ", 40), "monster")
	if len(long) > filenameLimit+len(".md") {
		t.Errorf("Filename gave %d characters, want no more than %d", len(long), filenameLimit)
	}
	if strings.HasSuffix(strings.TrimSuffix(long, ".md"), "-") {
		t.Errorf("a truncated filename ends in a dash: %q", long)
	}
}

// THE TWO STRUCTS THIS PACKAGE WALKS ARE THE PAGES' OWN, and a field added to
// either is a field the export will silently not write. Neither compiler nor
// test would notice: the file just comes out missing a section.
//
// So the field lists are written down here, and adding one is a failing test
// with a one-line fix -- either export it or say in this list that it is not
// exported. It is the same reflect check the password gate uses, and for the
// same reason: what a type is allowed to carry is the assertion.
func TestTheExportedTypesHaveNotGrownAFieldNobodyExports(t *testing.T) {
	for name, c := range map[string]struct {
		fields any
		want   []string
	}{
		"StatBlock": {pages.StatBlock{}, []string{
			"Name", "Subtitle", "Image", "AC", "Initiative", "HP", "HitDice", "Speed",
			"Abilities", "Lines", "Sections", "Habitat", "Treasure",
		}},
		"SharedCharacterSheet": {pages.SharedCharacterSheet{}, []string{
			"Header", "Avatar", "Identity", "CoreStats", "Spellcasting", "Vitals",
			"Abilities", "SavingThrows", "Skills", "PassivePerception", "Training",
			"Attacks", "Features", "Equipped", "SpellSlots", "Prepared",
			"Personality", "Appearance", "Actions",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			names := []string{}
			for _, field := range reflect.VisibleFields(reflect.TypeOf(c.fields)) {
				if field.Anonymous {
					continue
				}
				names = append(names, field.Name)
			}

			if !slices.Equal(names, c.want) {
				t.Errorf("%s carries %v, want %v -- a new field is either exported or listed here on purpose", name, names, c.want)
			}
		})
	}
}
