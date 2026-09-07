// Package export writes a monster or a character out as one Markdown file: the
// thing a GM downloads to paste into an AI tool or drop into an Obsidian vault.
//
// IT TAKES THE PAGE'S OWN STRUCTS AND NOTHING ELSE. pages.StatBlock is what the
// editor, the View dialog and the shared page all render; pages.SharedCharacterSheet
// is what a shared sheet renders. Building the file from those means the export
// and the page cannot disagree about what a monster is -- a column that reaches
// one reaches the other, and a value the page decided to leave out is not in the
// file either. It is also the whole of why an owner exporting their own monster
// and a stranger exporting it from a link get byte-identical files.
//
// THE FILE IS THE PAGE, IN THE PAGE'S ORDER. Every section here is a panel up
// there, in the same sequence, and an empty one is not written at all -- the doc
// builder drops empty blocks, which is the same rule the markup follows when it
// refuses to render an Attacks panel for a character with no attacks.
//
// WHAT IS DELIBERATELY NOT IN THE FILE:
//
//   - Pictures. Every image URL in this app needs either a session or a share
//     token, so a reference to one would be a broken image in the reader's
//     vault forever. A stat block is text and it survives being text.
//   - Ids. Nothing here names a monster, a character, an asset or an owner. The
//     file is about the creature rather than about the row, so a vault full of
//     these carries nothing that could be pasted back at the app.
//
// THE FRONTMATTER IS PROPERTIES AND THE BODY IS THE SHEET. Obsidian reads the
// block at the top as fields to filter and sort on, and a model reads it as a
// summary before the detail. Every value in it is written as a quoted string --
// no numbers, no bare words -- because "1/4" is a real challenge rating, "31/38"
// is real hit points, and a YAML parser guessing at either is worse than one
// that never guesses.
package export

import (
	"strings"

	"tabletopper/templ/pages"
)

// Monster is the stat block as the page renders it, plus the GM's own paragraph
// -- which is not part of the block and is a panel of its own on the shared
// page, so it is a second argument here rather than a field on StatBlock.
func Monster(block pages.StatBlock, description string) []byte {
	name := monsterName(block.Name)

	var d doc
	d.add(frontmatter(
		field{"title", name},
		field{"type", "monster"},
		field{"subtitle", block.Subtitle},
		field{"armor_class", block.AC},
		field{"hit_points", block.HP},
		field{"challenge_rating", lineValue(block.Lines, "CR")},
		tagField("monster"),
	))

	d.add("# " + name)
	if block.Subtitle != "" {
		d.add("*" + block.Subtitle + "*")
	}

	// The block's lead, which is the one place a value appears in the body and
	// in the frontmatter both. That is the convention a vault expects: the
	// properties are for filtering, the block is for reading.
	d.add(bullets(
		fact{"AC", block.AC},
		fact{"Initiative", block.Initiative},
		fact{"HP", hitPoints(block)},
		fact{"Speed", block.Speed},
	))

	d.add(abilityTable(block.Abilities))

	lines := make([]fact, 0, len(block.Lines))
	for _, line := range block.Lines {
		lines = append(lines, fact{line.Label, line.Value})
	}
	d.add(bullets(lines...))

	for _, section := range block.Sections {
		d.add("## " + section.Heading)
		if section.Intro != "" {
			d.add("*" + section.Intro + "*")
		}
		for _, entry := range section.Entries {
			d.add(entryParagraph(entry.Label, entry.Value))
		}
	}

	// The two lines the book prints under the block rather than in it. They get
	// a heading here that the page does not give them: on screen a rule and a
	// gap say "this is no longer the stat block", and in a file the only thing
	// that can say it is a heading -- without one they read as part of whatever
	// section happened to come last.
	d.section("Habitat and Treasure", bullets(
		fact{"Habitat", block.Habitat},
		fact{"Treasure", block.Treasure},
	))

	if strings.TrimSpace(description) != "" {
		d.add("## Description")
		d.add(strings.TrimSpace(description))
	}

	return d.bytes()
}

// Character is the shared sheet: the Character tab, read-only, in its own panel
// order.
//
// THE SIX BAR READINGS ARE IN THE FRONTMATTER AND NOWHERE ELSE. The page prints
// them twice on purpose -- as chips across the top and again inside the panels
// -- because a chip is glanceable and a panel is complete. A file has no glance,
// so printing armour class twice would be two lines for a model to reconcile;
// the properties block does that job instead.
func Character(sheet pages.SharedCharacterSheet) []byte {
	header := sheet.Header
	name := characterName(header.Name)

	var d doc
	d.add(frontmatter(
		field{"title", name},
		field{"type", "character"},
		field{"subtitle", header.Subtitle},
		field{"armor_class", header.AC},
		field{"hit_points", hitPointRange(header)},
		field{"speed", header.Speed},
		field{"initiative", header.Initiative},
		field{"proficiency_bonus", header.Proficiency},
		field{"passive_perception", header.Passive},
		tagField("character"),
	))

	d.add("# " + name)
	if header.Subtitle != "" {
		d.add("*" + header.Subtitle + "*")
	}

	// THE HEADINGS ARE THE PAGE'S PANEL NAMES, word for word, so a reader who
	// has seen the sheet is looking at the same fourteen boxes in a column.
	// What is not the page's is the sequence: on screen the panels are dealt
	// into two columns, which puts the ability scores above the character's own
	// name, and a file has one column and is read top to bottom. So this is the
	// order a sheet is read in -- who they are, what they can do, what they
	// carry -- and it is the only thing here that is not taken from the markup.
	//
	// Spellcasting is folded into Core Stats rather than given a heading,
	// because that is where the page puts it: a strip inside that panel, for a
	// character who casts and absent for one who does not.
	d.section("Identity", factBullets(sheet.Identity))
	d.section("Core Stats", factBullets(append(append([]pages.SharedFact{}, sheet.CoreStats...), sheet.Spellcasting...)))
	d.section("Vitals", factBullets(sheet.Vitals))

	if len(sheet.Abilities) > 0 {
		rows := make([][]string, 0, len(sheet.Abilities))
		for _, ability := range sheet.Abilities {
			rows = append(rows, []string{ability.Label, ability.Score, ability.Mod})
		}
		d.section("Abilities", table([]string{"Ability", "Score", "Mod"}, rows))
	}

	d.section("Saving Throws", bonusTable("Save", sheet.SavingThrows))
	d.section("Skills", bonusTable("Skill", sheet.Skills))
	if sheet.PassivePerception != "" {
		d.add(bullets(fact{"Passive Perception", sheet.PassivePerception}))
	}

	d.section("Proficiencies & Training", factBullets(sheet.Training))
	d.section("Attacks", attackTable(sheet.Attacks))
	// The notes a row carries do not fit a cell -- they are prose and a table
	// cell cannot hold a newline -- so they follow the table as paragraphs, and
	// only for the rows that have any.
	for _, attack := range sheet.Attacks {
		if strings.TrimSpace(attack.Notes) != "" {
			d.add(entryParagraph(attack.Name, attack.Notes))
		}
	}

	if len(sheet.Features) > 0 {
		d.add("## Features & Traits")
		for _, feature := range sheet.Features {
			d.add(entryParagraph(feature.Label, feature.Value))
		}
	}

	if len(sheet.Equipped) > 0 {
		items := make([]string, 0, len(sheet.Equipped))
		for _, item := range sheet.Equipped {
			items = append(items, "- "+bold(itemName(item))+description(item.Description))
		}
		d.section("Equipment", strings.Join(items, "\n"))
	}

	if len(sheet.SpellSlots) > 0 {
		rows := make([][]string, 0, len(sheet.SpellSlots))
		for _, level := range sheet.SpellSlots {
			rows = append(rows, []string{level.Name, level.Slots, level.Spells})
		}
		d.section("Spell Slots", table([]string{"Level", "Slots", "Prepared"}, rows))
	}

	// The slots come first and the spells after them, which is the one place
	// the two-column layout had them the other way round. What a caster can do
	// tonight is the table; the spells are the detail under it.
	if len(sheet.Prepared) > 0 {
		d.add("## Prepared Spells")
		for _, group := range sheet.Prepared {
			if len(group.Spells) == 0 {
				continue
			}

			d.add("### " + group.Name)
			for _, spell := range group.Spells {
				d.add(spellParagraph(spell))
			}
		}
	}

	if len(sheet.Personality) > 0 {
		d.add("## Personality")
		for _, trait := range sheet.Personality {
			d.add(entryParagraph(trait.Label, trait.Value))
		}
	}

	d.section("Appearance", factBullets(sheet.Appearance))

	return d.bytes()
}

// Filename is what the browser saves the file as, and it is a slug rather than
// the name itself: a monster may be called "Kuo-toa Whip (Sea)" or be written in
// a script this app never has to think about, and a filename in a header is the
// one place either would have to be escaped. Everything outside a-z0-9 becomes a
// dash, so nothing that reaches Content-Disposition can carry a quote, a
// semicolon or a byte over 127 -- and a name that slugs away to nothing falls
// back to the word for what it is.
func Filename(name, fallback string) string {
	var slug strings.Builder
	dash := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			slug.WriteRune(r)
			dash = false
		case !dash && slug.Len() > 0:
			slug.WriteByte('-')
			dash = true
		}
		if slug.Len() >= filenameLimit {
			break
		}
	}

	name = strings.Trim(slug.String(), "-")
	if name == "" {
		name = fallback
	}

	return name + ".md"
}

// filenameLimit keeps a long name from becoming a long filename. It is well
// under every filesystem's own limit, because the point is a name somebody can
// read in a downloads list rather than one that merely saves.
const filenameLimit = 60
