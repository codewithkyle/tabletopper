package export
import (
	"strings"
	"tabletopper/templ/pages"
)
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
const filenameLimit = 60
