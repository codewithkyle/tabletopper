package pages

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func TestTheSharedEntrysBodyIsTheOnlyMarkupOnThePage(t *testing.T) {
	body := renderToString(t, SharedJournalEntry(SharedJournalData{
		Character: SharedCharacter{Name: "<script>", Classes: "Fighter", Race: "Orc", Level: "3"},
		Title:     "The road to <b>Phandalin</b>",
		Body:      "<p>We <em>walked</em>.</p>",
	}))
	if !strings.Contains(body, "<p>We <em>walked</em>.</p>") {
		t.Errorf("the rendered body was escaped rather than written:\n%s", body)
	}
	if strings.Contains(body, "<b>Phandalin</b>") {
		t.Errorf("the title was written as markup:\n%s", body)
	}
	if strings.Contains(body, "<script>") {
		t.Errorf("a character name was written as markup:\n%s", body)
	}
}
func TestTheSharedPagesShipNoScriptsAndNoDialogs(t *testing.T) {
	pages := map[string]templ.Component{
		"entry":   SharedJournalEntry(SharedJournalData{}),
		"sheet":   SharedCharacterPage(testSharedSheet()),
		"monster": SharedMonsterPage(testSharedMonster()),
		"locked":  ShareLocked(ShareLockedData{Action: "/share/tok"}),
		"dead":    ShareUnavailable(),
	}
	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, page)
			for _, forbidden := range []string{"<script", "<dialog", "htmx", "hx-"} {
				if strings.Contains(body, forbidden) {
					t.Errorf("a shared page carries %q:\n%s", forbidden, body)
				}
			}
			if !strings.Contains(body, `name="robots" content="noindex, nofollow"`) {
				t.Errorf("a shared page is missing its robots meta:\n%s", body)
			}
		})
	}
}
func TestThePasswordGateIsHandedNothingItCouldLeak(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(ShareLockedData{}))
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	if want := []string{"Action", "Problem"}; !slices.Equal(names, want) {
		t.Errorf("ShareLockedData carries %v, want %v -- see the comment above", names, want)
	}
}
func TestThePasswordGateAsksForAPassword(t *testing.T) {
	body := renderToString(t, ShareLocked(ShareLockedData{Action: "/share/tok"}))
	if !strings.Contains(body, `action="/share/tok"`) {
		t.Errorf("the form does not post to the share:\n%s", body)
	}
	if !strings.Contains(body, `type="password"`) {
		t.Errorf("there is no password field:\n%s", body)
	}
	if !strings.Contains(body, `method="post"`) {
		t.Errorf("the gate is not a plain form post:\n%s", body)
	}
}
func TestAnUnnamedEntryStillHasATitle(t *testing.T) {
	if got := ShareTitle("   "); !strings.HasPrefix(got, "Untitled entry") {
		t.Errorf("ShareTitle(blank) = %q", got)
	}
	body := renderToString(t, SharedJournalEntry(SharedJournalData{}))
	if !strings.Contains(body, "Untitled entry") {
		t.Errorf("an unnamed entry rendered no heading:\n%s", body)
	}
}
func TestTheShareDialogShowsTheFormOrTheLinkAndNeverBoth(t *testing.T) {
	for name, action := range map[string]string{
		"journal":   "/characters/C/journal/E/share",
		"character": "/characters/C/share",
		"monster":   "/monsters/M/share",
	} {
		t.Run(name, func(t *testing.T) {
			form := renderToString(t, ShareDialog(ShareDialogData{Action: action}))
			if !strings.Contains(form, "Create link") || strings.Contains(form, "Revoke link") {
				t.Errorf("an unshared thing did not render the form alone:\n%s", form)
			}
			if !strings.Contains(form, `hx-post="`+action+`"`) {
				t.Errorf("the form does not post to the share route:\n%s", form)
			}
			link := renderToString(t, ShareDialog(ShareDialogData{
				Action: action, Link: "https://tabletopper.test/share/tok",
			}))
			if !strings.Contains(link, "Revoke link") || strings.Contains(link, "Create link") {
				t.Errorf("a shared thing did not render the link alone:\n%s", link)
			}
			if !strings.Contains(link, `value="https://tabletopper.test/share/tok"`) {
				t.Errorf("the link is not in the field to copy:\n%s", link)
			}
			if !strings.Contains(link, `hx-delete="`+action+`"`) {
				t.Errorf("revoke does not name the share route:\n%s", link)
			}
		})
	}
}
func TestTheDialogSaysWhatKindOfLinkItIs(t *testing.T) {
	cases := map[string]struct {
		data ShareDialogData
		want string
	}{
		"never expires": {ShareDialogData{Link: "u"}, "Never expires."},
		"expires later": {
			ShareDialogData{Link: "u", Expires: Timestamp{ISO: "2026-09-13T00:00:00Z", Text: "13 Sep 2026"}},
			"13 Sep 2026",
		},
		"already expired": {
			ShareDialogData{Link: "u", Expired: true, Expires: Timestamp{ISO: "2026-09-01T00:00:00Z"}},
			"expired",
		},
		"password":    {ShareDialogData{Link: "u", Protected: true}, "password is required"},
		"no password": {ShareDialogData{Link: "u"}, "anyone with the link"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, ShareDialog(tc.data))
			if !strings.Contains(body, tc.want) {
				t.Errorf("wanted %q in the dialog:\n%s", tc.want, body)
			}
		})
	}
}
func TestTheShareButtonNamesAFragmentRoute(t *testing.T) {
	for name, c := range map[string]struct {
		page templ.Component
		want string
	}{
		"entry":     {EditCharacterJournalEntry(JournalEntryPageData{CharacterID: "C", EntryID: "E"}), `data-modal-open="/fragment/character/journal-share?character=C&amp;entry=E"`},
		"character": {EditCharacter(EditCharacterPageData{CharacterID: "C"}), `data-modal-open="/fragment/character/share?character=C"`},
		"monster":   {EditMonster(EditMonsterPageData{MonsterID: "M", Header: MonsterHeader{MonsterID: "M"}}), `data-modal-open="/fragment/monster/share?monster=M"`},
	} {
		t.Run(name, func(t *testing.T) {
			if body := renderToString(t, c.page); !strings.Contains(body, c.want) {
				t.Errorf("no share button opens %s:\n%s", c.want, body)
			}
		})
	}
}
func TestEachEditorPageHasOneMeaningOfShare(t *testing.T) {
	for name, page := range map[string]templ.Component{
		"character": EditCharacter(EditCharacterPageData{CharacterID: "C"}),
		"journal":   EditCharacterJournal(JournalPageData{CharacterID: "C"}),
		"monster":   EditMonster(EditMonsterPageData{MonsterID: "M", Header: MonsterHeader{MonsterID: "M"}}),
	} {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, page)
			if !strings.Contains(body, ">Share<") {
				t.Errorf("the %s bar does not say Share:\n%s", name, body)
			}
			if strings.Contains(body, ">Share character<") || strings.Contains(body, ">Share monster<") {
				t.Errorf("the %s bar spends words on a noun the page already carries:\n%s", name, body)
			}
		})
	}
	entry := renderToString(t, EditCharacterJournalEntry(JournalEntryPageData{CharacterID: "C", EntryID: "E"}))
	if !strings.Contains(entry, ">Share entry<") {
		t.Errorf("the entry page lost the share that means the entry:\n%s", entry)
	}
	if strings.Contains(entry, characterShareDialogURL("C")) {
		t.Errorf("the entry page still opens the character's share dialog:\n%s", entry)
	}
	if strings.Count(entry, "data-modal-open") != 1 {
		t.Errorf("the entry page opens %d dialogs from its bar, want one:\n%s", strings.Count(entry, "data-modal-open"), entry)
	}
}
func TestTheShareButtonsKeepTheirLabelsWhenTheyCollapse(t *testing.T) {
	body := renderToString(t, EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: "C", EntryID: "E",
	}))
	for _, label := range []string{"Share entry", "Export"} {
		if !strings.Contains(body, `class="max-[640px]:sr-only">`+label+`<`) {
			t.Errorf("%q is not hidden with sr-only:\n%s", label, body)
		}
		if strings.Contains(body, `max-[640px]:hidden">`+label+`<`) {
			t.Errorf("%q is hidden from assistive technology too:\n%s", label, body)
		}
	}
	if !strings.Contains(body, ">Save<") {
		t.Errorf("the bar lost Save:\n%s", body)
	}
}
func TestEverySurfaceOffersTheMarkdownExport(t *testing.T) {
	for name, c := range map[string]struct {
		page  templ.Component
		want  string
		label string
	}{
		"monster editor":   {EditMonster(EditMonsterPageData{MonsterID: "M", Header: MonsterHeader{MonsterID: "M"}}), "/monsters/M/export.md", "Export"},
		"character editor": {EditCharacter(EditCharacterPageData{CharacterID: "C"}), "/characters/C/export.md", "Export"},
		"journal tab":      {EditCharacterJournal(JournalPageData{CharacterID: "C"}), "/characters/C/export.md", "Export"},
		"shared monster":   {SharedMonsterPage(testSharedMonster()), "/share/tok/export.md", "Export Markdown"},
		"shared sheet":     {SharedCharacterPage(testSharedSheet()), "/share/tok/export.md", "Export Markdown"},
	} {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, c.page)
			if !strings.Contains(body, `href="`+c.want+`" download`) {
				t.Errorf("no download link to %s:\n%s", c.want, body)
			}
			if !strings.Contains(body, ">"+c.label+"<") {
				t.Errorf("the button does not say %q:\n%s", c.label, body)
			}
		})
	}
}
func TestTheMonsterEditorCarriesAShareButtonAndTheManualDoesNot(t *testing.T) {
	editor := renderToString(t, EditMonster(EditMonsterPageData{
		MonsterID: "M",
		Header:    MonsterHeader{MonsterID: "M", Name: "Goblin"},
	}))
	if !strings.Contains(editor, `data-modal-open="/fragment/monster/share?monster=M"`) {
		t.Errorf("the monster editor has no share button:\n%s", editor)
	}
	if !strings.Contains(editor, `class="max-[640px]:sr-only">Share<`) {
		t.Errorf("the label is not hidden with sr-only:\n%s", editor)
	}
	manual := renderToString(t, Monsters(MonsterListData{Monsters: []MonsterSummary{testMonsterCard()}}))
	if strings.Contains(manual, "/fragment/monster/share") {
		t.Errorf("a manual card carries a share button:\n%s", manual)
	}
}
func TestEveryEditorTabButTheEntryCarriesTheCharacterShareButton(t *testing.T) {
	tabs := map[string]struct {
		page   templ.Component
		shares bool
	}{
		"character": {EditCharacter(EditCharacterPageData{CharacterID: "C"}), true},
		"inventory": {EditCharacterInventory(InventoryPageData{CharacterID: "C"}), true},
		"spells":    {EditCharacterSpellLevel(SpellLevelPageData{CharacterID: "C"}), true},
		"journal":   {EditCharacterJournal(JournalPageData{CharacterID: "C"}), true},
		"entry":     {EditCharacterJournalEntry(JournalEntryPageData{CharacterID: "C", EntryID: "E"}), false},
	}
	for name, tab := range tabs {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, tab.page)
			if got := strings.Contains(body, `data-modal-open="/fragment/character/share?character=C"`); got != tab.shares {
				if tab.shares {
					t.Errorf("the %s tab has no character share button:\n%s", name, body)
				} else {
					t.Errorf("the %s tab draws a share that is not the one it means:\n%s", name, body)
				}
			}
		})
	}
}
