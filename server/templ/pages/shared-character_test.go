package pages

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// A sheet with something in every section, so the tests below are looking at
// markup that was actually produced rather than at branches that were skipped.
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
		Skills:            []SharedBonus{{Label: "Stealth", Abbr: "DEX", Total: "+9"}},
		PassivePerception: "15",
		Training:          []SharedFact{{Label: "Languages", Value: "Common, Elvish"}},
		Attacks:           []SharedAttack{{Name: "Longbow", Bonus: "+8", Damage: "1d8+5", Mastery: "Slow"}},
		Features:          []SharedFact{{Label: "Favoured Enemy", Value: "Undead"}},
		Equipped:          []SharedItem{{Name: "Studded Leather", Quantity: "2"}},
		SpellSlots:        []SharedSpellLevel{{Name: "Level 1", Slots: "4 slots", Spells: "3 spells"}},
		Prepared: []SharedSpellGroup{{
			Name:   "Level 1",
			Spells: []SharedSpell{{Name: "Hunter's Mark", Meta: "Bonus action | 90 feet"}},
		}},
		Personality: []SharedFact{{Label: "Bonds", Value: "My brother."}},
		Appearance:  []SharedFact{{Label: "Eyes", Value: "Grey"}},
	}
}

// A public page carries no session-shaped machinery: no htmx, no dialogs, no
// modal modules. It is the whole reason there is a second layout, and the sheet
// is the harder half of it -- the editor's version of every panel here is a form
// that autosaves.
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

// A SHARED PAGE POINTS ONLY WHERE IT MEANS TO, which is what "it links nowhere
// into the app" comes to in markup. The editor's read-only panels -- equipped
// items, prepared spells, the spell slot list -- each link back to the tab that
// fills them, and those routes need a session, so reusing one here would put a
// link on a stranger's page that answers with a redirect to a sign-in form.
//
// THE PAGE USED TO CARRY NO ANCHOR AT ALL and the Markdown export is why it now
// does. That is a widening of the rule rather than a break in it: the download
// is under /share/ like every other URL a reader is handed, so it is reachable
// by exactly whoever the page is and by nobody else. The shared monster page
// adds one more, /sign-in, and that one is the whole point of it -- a reader
// with no account is being told where an account comes from.
//
// Withholding the character id from SharedCharacterSheet is what makes the rest
// true; see the test below. This is the check that nobody has reintroduced a
// link some other way.
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

// anchorHrefs is every href the page renders. It is a regexp rather than a
// parser because the assertion is about what is in the markup, and a page with
// an anchor this pattern cannot see is a page that has stopped being simple
// enough for this rule to be checkable.
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

// THE ASSERTION IS ON THE TYPE RATHER THAN ON THE MARKUP, the way the password
// gate's is: no id means no component on this page can be handed one, so a link
// into the app is not a rule to remember but an argument that does not exist.
func TestASharedSheetIsHandedNoCharacterID(t *testing.T) {
	for _, field := range reflect.VisibleFields(reflect.TypeOf(SharedCharacterSheet{})) {
		if strings.Contains(field.Name, "ID") {
			t.Errorf("SharedCharacterSheet carries %s -- see the comment above", field.Name)
		}
	}
}

// The one omission that was asked for by name. How many slots a character has is
// a fact about them; how many are left is where they are in tonight's session,
// and a link pasted into a chat window is read hours after it was sent.
func TestASharedSpellLevelCarriesNoUsedCount(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(SharedSpellLevel{}))

	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	if want := []string{"Name", "Slots", "Spells"}; !slices.Equal(names, want) {
		t.Errorf("SharedSpellLevel carries %v, want %v -- see the comment above", names, want)
	}
}

// An empty section is not rendered at all. The editor shows a box for every
// field because it is talking to the person who can fill it in; a reader cannot,
// so a character with no attacks has no Attacks panel rather than an empty one.
func TestASheetWithNothingInItRendersNoEmptyPanels(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(SharedCharacterSheet{}))

	for _, heading := range []string{
		"Abilities", "Saving Throws", "Identity", "Core Stats", "Vitals", "Attacks",
		"Proficiencies &amp; Training", "Skills", "Features &amp; Traits", "Equipment",
		"Prepared Spells", "Spell Slots", "Personality", "Appearance",
	} {
		if strings.Contains(body, heading) {
			t.Errorf("an empty sheet rendered the %s panel:\n%s", heading, body)
		}
	}
}

// What a reader is actually shown, once there is something to show. The six bar
// readings come from the same CharacterHeader the editor builds, so this is also
// what says the shared sheet and the editor cannot disagree about an initiative.
func TestASharedSheetRendersTheCharacterTabsPanels(t *testing.T) {
	body := renderToString(t, SharedCharacterPage(testSharedSheet()))

	for _, want := range []string{
		"Vex", "Half-Elf | Ranger 5", `src="/share/tok/portrait"`,
		"Longbow", "1d8+5", "Slow",
		"Studded Leather", "Hunter&#39;s Mark", "4 slots",
		"Favoured Enemy", "Undead", "Grey", "My brother.",
		"Stealth", "+9", "Passive Perception",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the sheet does not show %q:\n%s", want, body)
		}
	}
}

// An unnamed character still has a heading and a tab title, the way an unnamed
// entry does. A blank line where either goes reads as a rendering bug.
func TestAnUnnamedSharedCharacterStillHasATitle(t *testing.T) {
	if got := SharedCharacterTitle("   "); !strings.HasPrefix(got, "Unnamed character") {
		t.Errorf("SharedCharacterTitle(blank) = %q", got)
	}

	body := renderToString(t, SharedCharacterPage(SharedCharacterSheet{}))
	if !strings.Contains(body, "Unnamed character") {
		t.Errorf("an unnamed character rendered no heading:\n%s", body)
	}
}
