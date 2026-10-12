package pages

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func testSharedSheet() SharedCharacterSheet {
	return SharedCharacterSheet{
		Header: CharacterHeader{
			Name:        "Vex",
			Subtitle:    "Half-Elf | Ranger 5",
			AC:          "16",
			CurrentHP:   "31",
			MaxHP:       "38",
			Speed:       "30 ft.",
			Initiative:  "+3",
			Proficiency: "+3",
			Passive:     "15",
		},
		Actions:           SharedActions{Export: "/share/tok/export.md"},
		Avatar:            "/share/tok/portrait",
		Identity:          []SharedFact{{Label: "Species", Value: "Half-Elf"}},
		CoreStats:         []SharedFact{{Label: "Armor Class", Value: "16"}},
		Spellcasting:      []SharedFact{{Label: "Spell Save DC", Value: "14"}},
		Vitals:            []SharedFact{{Label: "Current Hit Points", Value: "31"}},
		Abilities:         []SharedAbility{{Label: "Strength", Score: "12", Mod: "+1"}},
		SavingThrows:      []SharedBonus{{Label: "Dexterity", Abbr: "DEX", Total: "+6"}},
		Skills:            []SharedBonus{{Label: "Stealth", Abbr: "DEX", Proficiency: "Expertise", Total: "+9"}},
		PassivePerception: "15",
		Training:          []SharedFact{{Label: "Languages", Value: "Common, Elvish"}},
		Attacks:           []SharedAttack{{Name: "Longbow", Bonus: "+8", Damage: "1d8+5", Mastery: "Slow"}},
		Features:          []SharedFact{{Label: "Favoured Enemy", Value: "Undead"}},
		Inventory: []SharedItem{
			{Name: "Studded Leather", Quantity: "1", Value: "45 gp", Weight: "13", Equipped: true},
			{Name: "Hempen Rope", Quantity: "2", Value: "1 gp", Weight: "10", Description: "Fifty feet, knotted every ten."},
		},
		Spells: []SharedSpellLevel{{
			Name:  "Level 1",
			Slots: "3 of 4 slots left",
			Spells: []SharedSpell{
				{Name: "Hunter's Mark", Prepared: true, School: "Divination", CastingTime: "Bonus action", Range: "90 feet", Duration: "1 hour", Components: "V", Description: "You choose a creature you can see."},
				{Name: "Ensnaring Strike", School: "Conjuration", CastingTime: "Bonus action"},
			},
		}},
		Personality: []SharedFact{{Label: "Bonds", Value: "My brother."}},
		Appearance:  []SharedFact{{Label: "Eyes", Value: "Grey"}},
	}
}
func TestASharedSheetShipsNoScriptsAndNoDialogs(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(testSharedSheet()))
	for _, forbidden := range []string{"<script", "<dialog", "htmx", "hx-post", "hx-get", "<form", "<input"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("a shared sheet carries %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, `name="robots" content="noindex, nofollow"`) {
		t.Errorf("a shared sheet is missing its robots meta:\n%s", body)
	}
}
func TestASharedPageLinksNowhereIntoTheApp(t *testing.T) {
	for name, c := range map[string]struct {
		body    string
		allowed []string
	}{
		"sheet": {
			body:    renderToString(t, SharedCharacterPage(testSharedSheet())),
			allowed: []string{"/share/"},
		},
		"monster": {
			body:    renderToString(t, SharedMonsterPage(testSharedMonsterForGuest())),
			allowed: []string{"/share/", "/sign-in"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for _, href := range anchorHrefs(c.body) {
				if !hasAnyPrefix(href, c.allowed) {
					t.Errorf("a shared page links to %q, want one of %v", href, c.allowed)
				}
			}
		})
	}
	if body := renderToString(t, SharedCharacterPage(testSharedSheet())); strings.Contains(body, "/characters/") {
		t.Errorf("a shared sheet named a route inside the app:\n%s", body)
	}
}
func anchorHrefs(body string) []string {
	hrefs := []string{}
	for _, match := range regexp.MustCompile(`<a [^>]*href="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		hrefs = append(hrefs, match[1])
	}
	return hrefs
}
func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
func TestASharedSheetIsHandedNoCharacterID(t *testing.T) {
	for _, field := range reflect.VisibleFields(reflect.TypeOf(SharedCharacterSheet{})) {
		if strings.Contains(field.Name, "ID") {
			t.Errorf("SharedCharacterSheet carries %s -- see the comment above", field.Name)
		}
	}
}
func TestASheetWithNothingInItRendersNoEmptyPanels(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(SharedCharacterSheet{}))
	for _, heading := range []string{
		"Abilities", "Saving Throws", "Identity", "Core Stats", "Vitals", "Attacks",
		"Proficiencies &amp; Training", "Skills", "Features &amp; Traits", "Inventory",
		"Spells", "Personality", "Appearance",
	} {
		if strings.Contains(body, heading) {
			t.Errorf("an empty sheet rendered the %s panel:\n%s", heading, body)
		}
	}
}
func TestASharedSheetRendersTheCharacterTabsPanels(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(testSharedSheet()))
	for _, want := range []string{
		"Vex", "Half-Elf | Ranger 5", `src="/share/tok/portrait"`,
		"Longbow", "1d8+5", "Slow",
		"Studded Leather", "45 gp", "Hempen Rope", "Fifty feet, knotted every ten.",
		"Hunter&#39;s Mark", "Ensnaring Strike", "Divination", "90 feet", "You choose a creature you can see.",
		"3 of 4 slots left", "Expertise",
		"Favoured Enemy", "Undead", "Grey", "My brother.",
		"Stealth", "+9", "Passive Perception",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the sheet does not show %q:\n%s", want, body)
		}
	}
}
func TestAnUnnamedSharedCharacterStillHasATitle(t *testing.T) {
	if got := SharedCharacterTitle("   "); !strings.HasPrefix(got, "Unnamed character") {
		t.Errorf("SharedCharacterTitle(blank) = %q", got)
	}
	body := renderToString(t, SharedCharacterPage(SharedCharacterSheet{}))
	if !strings.Contains(body, "Unnamed character") {
		t.Errorf("an unnamed character rendered no heading:\n%s", body)
	}
}
func TestASharedSheetLetsAReaderOpenSeveralDetailsAtOnce(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(testSharedSheet()))
	if got := strings.Count(body, "<details"); got < 3 {
		t.Fatalf("%d details on the page, want one per spell and one per described item", got)
	}
	if strings.Contains(body, "<details name=") {
		t.Errorf("opening one spell on a shared sheet closes another:\n%s", body)
	}
}
