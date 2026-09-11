package export

import (
	"strings"

	"tabletopper/templ/pages"
)

type doc struct {
	blocks []string
}

func (d *doc) add(block string) {
	if strings.TrimSpace(block) != "" {
		d.blocks = append(d.blocks, block)
	}
}
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

type field struct {
	key   string
	value string
}

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
		default:
			quoted.WriteRune(r)
		}
	}
	quoted.WriteByte('"')
	return quoted.String()
}

type fact struct {
	label string
	value string
}

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
func entryParagraph(label, value string) string {
	value = strings.TrimSpace(value)
	switch {
	case label == "":
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
func cell(value string) string {
	return strings.ReplaceAll(oneLine(value), "|", "\\|")
}
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
func hitPoints(block pages.StatBlock) string {
	if block.HitDice == "" {
		return block.HP
	}
	return block.HP + " (" + block.HitDice + ")"
}
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
func lineValue(lines []pages.StatBlockEntry, label string) string {
	for _, line := range lines {
		if line.Label == label {
			return line.Value
		}
	}
	return ""
}
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
