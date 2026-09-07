package export

import (
	"strings"

	"tabletopper/templ/pages"
)

// The Markdown itself: a block builder and the handful of shapes both files are
// made of.
//
// A DOCUMENT IS A LIST OF BLOCKS JOINED BY BLANK LINES, which is the whole
// reason this type exists. Markdown's line handling is the thing that goes wrong
// in a generated file: two lines with a single newline between them are one
// paragraph, so a stat block written line by line renders as a paragraph of
// run-together sentences in Obsidian while looking perfectly correct in the
// source. Blocks make that unrepresentable -- there is exactly one blank line
// between any two things, always.
//
// AN EMPTY BLOCK IS DROPPED RATHER THAN WRITTEN, which is the page's own rule:
// a character with no attacks has no Attacks panel, and now no Attacks heading
// either. It is what lets the callers above ask for every section unconditionally
// instead of guarding each one.
type doc struct {
	blocks []string
}

func (d *doc) add(block string) {
	if strings.TrimSpace(block) != "" {
		d.blocks = append(d.blocks, block)
	}
}

// section writes a heading and its body, or neither. The heading is held back
// until the body turns out to have something in it, so a section that would have
// been empty leaves no trace -- which is the difference between a file that says
// what a character has and a file that lists what they have not.
func (d *doc) section(heading, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}

	d.add("## " + heading)
	d.add(body)
}

func (d *doc) bytes() []byte {
	return []byte(strings.Join(d.blocks, "\n\n") + "\n")
}

// field is one line of the properties block. Every value is written as a quoted
// string -- see the package comment for why nothing here is allowed to look like
// a number.
type field struct {
	key   string
	value string
}

// tagField is the one list-valued property, and it is what makes a vault full of
// these searchable: `tag:#tabletopper/monster` finds every stat block exported
// from here without matching anything a person wrote by hand.
func tagField(kind string) field {
	return field{key: "tags", value: "tabletopper/" + kind}
}

func frontmatter(fields ...field) string {
	lines := []string{"---"}
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			continue
		}

		if f.key == "tags" {
			lines = append(lines, "tags:", "  - "+yamlString(f.value))
			continue
		}

		lines = append(lines, f.key+": "+yamlString(f.value))
	}

	return strings.Join(append(lines, "---"), "\n")
}

// yamlString is the only escaping in this package, and it is here because this
// is the only part of the file a parser reads strictly. A name holding a quote
// would end the value early and leave the rest as broken YAML; a newline in one
// would end the line and turn whatever followed into a key. Both are written by
// people into VARCHARs that allow them.
//
// Double quotes rather than single, because YAML's double-quoted form is the one
// with backslash escapes -- a single-quoted string cannot carry a control
// character at all.
func yamlString(value string) string {
	var quoted strings.Builder
	quoted.WriteByte('"')

	for _, r := range value {
		switch {
		case r == '"' || r == '\\':
			quoted.WriteByte('\\')
			quoted.WriteRune(r)
		case r == '\n' || r == '\r' || r == '\t':
			quoted.WriteByte(' ')
		case r < 0x20:
			// Anything else below space has no business in a property and no
			// readable escape; dropping it is better than emitting a byte that
			// makes the block unparseable.
		default:
			quoted.WriteRune(r)
		}
	}

	quoted.WriteByte('"')

	return quoted.String()
}

// fact is a bold label and its text, which is what most of both files is made
// of: a bulleted list of readings.
type fact struct {
	label string
	value string
}

// bullets writes a list and drops the entries with nothing in them. A list is
// used rather than a run of lines for the reason doc gives -- lines separated by
// a single newline are one paragraph -- and rather than a table because these
// are label-and-value pairs of wildly different lengths, which a table sets in
// two ragged columns.
func bullets(facts ...fact) string {
	lines := make([]string, 0, len(facts))
	for _, f := range facts {
		if strings.TrimSpace(f.value) == "" {
			continue
		}

		lines = append(lines, "- "+bold(f.label)+" "+oneLine(f.value))
	}

	return strings.Join(lines, "\n")
}

func factBullets(facts []pages.SharedFact) string {
	converted := make([]fact, 0, len(facts))
	for _, f := range facts {
		converted = append(converted, fact{f.Label, f.Value})
	}

	return bullets(converted...)
}

// entryParagraph is the book's own shape for a named piece of prose -- "**Bite.**
// Melee Attack Roll: +17" -- and it is what a trait, an action, a feature, a
// personality trait and an attack's notes all render as.
//
// THE TEXT IS PASSED THROUGH AS IT WAS TYPED. It is prose in a TEXT column, and
// a GM who wrote a list or a bold word into a monster's description meant it to
// be a list or a bold word: this file is going into a Markdown vault, so
// escaping it would turn their formatting into literal asterisks. The label is
// the half that is interpolated into markup, and it is written on one line so a
// name holding a newline cannot break out of the bold.
func entryParagraph(label, value string) string {
	value = strings.TrimSpace(value)

	switch {
	case label == "":
		// A regional effect has no name -- the book prints those as a bare
		// list -- and the page renders it as a paragraph on its own.
		return value
	case value == "":
		return bold(label)
	default:
		return bold(label+".") + " " + value
	}
}

func spellParagraph(spell pages.SharedSpell) string {
	line := bold(spell.Name + ".")
	if meta := oneLine(spell.Meta); meta != "" {
		line += " *" + meta + "*"
	}
	if description := strings.TrimSpace(spell.Description); description != "" {
		line += "\n\n" + description
	}

	return line
}

func abilityTable(abilities []pages.StatBlockAbility) string {
	rows := make([][]string, 0, len(abilities))
	for _, ability := range abilities {
		rows = append(rows, []string{ability.Label, ability.Score, ability.Mod, ability.Save})
	}

	return table([]string{"Ability", "Score", "Mod", "Save"}, rows)
}

// bonusTable is the skills table and the saving throws table, and the difference
// between them is a column that is not there.
//
// ABBR IS EMPTY ON EVERY SAVING THROW AND SET ON EVERY SKILL -- see
// pages.BonusRow: the ability a row keys off is worth saying under Acrobatics
// and is not worth saying under Strength, which is the same word. The markup
// prints it only when it is set, and the equivalent in a table is dropping the
// column: a heading with eighteen empty cells under it is worse than no heading,
// because a model reading the file has to decide what the blanks mean.
func bonusTable(heading string, bonuses []pages.SharedBonus) string {
	keyed := false
	for _, bonus := range bonuses {
		keyed = keyed || strings.TrimSpace(bonus.Abbr) != ""
	}

	rows := make([][]string, 0, len(bonuses))
	for _, bonus := range bonuses {
		if keyed {
			rows = append(rows, []string{bonus.Label, bonus.Abbr, bonus.Total})
			continue
		}

		rows = append(rows, []string{bonus.Label, bonus.Total})
	}

	if keyed {
		return table([]string{heading, "Ability", "Bonus"}, rows)
	}

	return table([]string{heading, "Bonus"}, rows)
}

func attackTable(attacks []pages.SharedAttack) string {
	rows := make([][]string, 0, len(attacks))
	for _, attack := range attacks {
		rows = append(rows, []string{attack.Name, attack.Bonus, attack.Damage, attack.DamageType, attack.Mastery})
	}

	return table([]string{"Attack", "Bonus", "Damage", "Type", "Mastery"}, rows)
}

// table writes GitHub-flavoured Markdown, which is what both ends of this file
// read: Obsidian renders it, and a model given six ability scores as a table
// answers about them more reliably than one given six sentences.
//
// It answers empty for no rows, so the section above it disappears.
func table(headings []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, row(headings), row(dividers(len(headings))))
	for _, cells := range rows {
		lines = append(lines, row(cells))
	}

	return strings.Join(lines, "\n")
}

func dividers(count int) []string {
	cells := make([]string, count)
	for i := range cells {
		cells[i] = "---"
	}

	return cells
}

func row(cells []string) string {
	escaped := make([]string, 0, len(cells))
	for _, c := range cells {
		escaped = append(escaped, cell(c))
	}

	return "| " + strings.Join(escaped, " | ") + " |"
}

// cell is the one place a value is escaped for the body, because a table row is
// the one place the body's markup is positional: an unescaped pipe in a skill
// name would open a column that the divider row has no width for, and every
// cell after it in that row would land under the wrong heading.
func cell(value string) string {
	return strings.ReplaceAll(oneLine(value), "|", "\\|")
}

// oneLine flattens a value that has to sit inside a line of markup. Only the
// places where a newline would break the shape use it -- a bullet's label and
// value, a table cell, a spell's meta line. Prose paragraphs keep their newlines,
// because that is what they are.
func oneLine(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")

	return strings.TrimSpace(strings.Join(strings.Fields(value), " "))
}

func bold(text string) string {
	text = oneLine(text)
	if text == "" {
		return ""
	}

	return "**" + text + "**"
}

// hitPoints is the HP line with the hit dice in brackets, which is how the block
// prints it and how a GM reads it back.
func hitPoints(block pages.StatBlock) string {
	if block.HitDice == "" {
		return block.HP
	}

	return block.HP + " (" + block.HitDice + ")"
}

// hitPointRange is the character's equivalent: the two numbers the bar shows as
// one chip. Current over maximum, because on a sheet the pair is the reading.
func hitPointRange(header pages.CharacterHeader) string {
	if header.CurrentHP == "" || header.MaxHP == "" {
		return header.CurrentHP + header.MaxHP
	}

	return header.CurrentHP + "/" + header.MaxHP
}

func itemName(item pages.SharedItem) string {
	if item.Quantity == "" {
		return item.Name
	}

	return item.Name + " ×" + item.Quantity
}

func description(text string) string {
	if text = oneLine(text); text != "" {
		return " — " + text
	}

	return ""
}

// lineValue pulls one of the block's labelled lines out by name. The lines are a
// slice rather than fields because the block omits what a monster does not have
// -- so this is a lookup and not an accessor, and a monster whose line is missing
// gets a property that is missing too rather than an empty one.
func lineValue(lines []pages.StatBlockEntry, label string) string {
	for _, line := range lines {
		if line.Label == label {
			return line.Value
		}
	}

	return ""
}

// The two fallbacks, which are the page's own: a thing shared or exported before
// it was named still has to have a heading, and a blank one reads as a file that
// failed to generate rather than as an unnamed monster.
func monsterName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Unnamed monster"
	}

	return name
}

func characterName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Unnamed character"
	}

	return name
}
