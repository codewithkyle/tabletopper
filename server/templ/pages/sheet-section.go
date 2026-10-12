package pages

import "github.com/a-h/templ"

type shellLayout struct {
	Fill     bool
	Back     backTarget
	Actions  templ.Component
	SubNav   templ.Component
	OwnShare bool
}

func panelFormID(panel string) string {
	return "panel-" + panel
}

var autosaves = templ.Attributes{"hx-on:submit": "event.preventDefault()"}

func sectionLayout(characterID string, section string) shellLayout {
	switch section {
	case SheetSectionAttacks:
		return shellLayout{Actions: attackAddButton(characterID, "btn btn-primary max-[640px]:flex-1")}
	case SheetSectionSpells:
		return shellLayout{SubNav: spellLevelTabs(characterID, SpellsPrepared)}
	}
	return shellLayout{}
}
