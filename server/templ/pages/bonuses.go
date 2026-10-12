package pages

import "tabletopper/internal/dnd"

var proficiencyOptions = dnd.Proficiencies()

type BonusRow struct {
	Key         string
	Label       string
	Abbr        string
	Proficiency string
	Misc        string
	Total       string
}
