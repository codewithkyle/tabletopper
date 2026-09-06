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

// The shared page is the one place in the app that writes a string to the
// response without escaping it, so the test that it does is also the test that
// says which string that is. internal/markdown is what makes that safe -- see
// its package comment -- and this pins the other half: the body reaches the
// document as markup, and every field beside it does not.
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

// A public page carries no session-shaped machinery: no htmx, no dialogs, no
// modal modules. It is the whole reason there is a second layout.
func TestTheSharedPageShipsNoScriptsAndNoDialogs(t *testing.T) {
	body := renderToString(t, SharedJournalEntry(SharedJournalData{}))

	for _, forbidden := range []string{"<script", "<dialog", "htmx"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("a shared page carries %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, `name="robots" content="noindex, nofollow"`) {
		t.Errorf("a shared page is missing its robots meta:\n%s", body)
	}
}

// The gate names nothing behind it. The password is there to keep the entry
// from being read by whoever finds the link, and a gate announcing whose
// journal it was guarding would give away part of the answer before asking the
// question.
//
// THE ASSERTION IS ON THE TYPE RATHER THAN ON THE MARKUP, because the type is
// what makes it true. The template cannot print an entry title or a character
// name it was never handed, so the two fields below are the whole guarantee --
// and a third one added here is exactly the change that should have to be
// argued for.
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

// An entry shared before it was named still has to render a heading and a tab
// title, and a blank line where either goes reads as a rendering bug.
func TestAnUnnamedEntryStillHasATitle(t *testing.T) {
	if got := ShareTitle("   "); !strings.HasPrefix(got, "Untitled entry") {
		t.Errorf("ShareTitle(blank) = %q", got)
	}

	body := renderToString(t, SharedJournalEntry(SharedJournalData{}))
	if !strings.Contains(body, "Untitled entry") {
		t.Errorf("an unnamed entry rendered no heading:\n%s", body)
	}
}

// The dialog is one component in two states, and Link is what picks. Nothing
// else decides, so the two can never both render.
//
// IT IS ALSO ONE COMPONENT FOR BOTH KINDS OF SHARE, so the same two states are
// checked against an entry's action URL and a character's -- the create and the
// revoke both come off Action, and a dialog whose revoke named a different row
// than its create is the one mistake that shape rules out.
func TestTheShareDialogShowsTheFormOrTheLinkAndNeverBoth(t *testing.T) {
	for name, action := range map[string]string{
		"journal":   "/characters/C/journal/E/share",
		"character": "/characters/C/share",
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

// The three sentences under the link, and the one that matters is the first:
// the row outlives its expiry by a day so the owner can see what happened, so
// the dialog has to say the link is dead rather than offer a URL that 404s.
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

// The Share button opens the dialog through the modal's declarative opener,
// and the URL it names has to be a /fragment/ route -- content-modal.js refuses
// anything else and the dialog would simply never open.
func TestTheShareButtonNamesAFragmentRoute(t *testing.T) {
	body := renderToString(t, EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: "C", EntryID: "E",
	}))

	for _, want := range []string{
		`data-modal-open="/fragment/character/journal-share?character=C&amp;entry=E"`,
		`data-modal-open="/fragment/character/share?character=C"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("no share button opens %s:\n%s", want, body)
		}
	}
}

// The character's Share button is on the bar rather than on a page, which is
// what puts it on all five editor tabs without any of them naming it. Losing
// that is losing the button from four pages at once, and nothing else would
// fail.
func TestEveryEditorTabCarriesTheCharacterShareButton(t *testing.T) {
	tabs := map[string]templ.Component{
		"character": EditCharacter(EditCharacterPageData{CharacterID: "C"}),
		"inventory": EditCharacterInventory(InventoryPageData{CharacterID: "C"}),
		"spells":    EditCharacterSpellLevel(SpellLevelPageData{CharacterID: "C"}),
		"journal":   EditCharacterJournal(JournalPageData{CharacterID: "C"}),
		"entry":     EditCharacterJournalEntry(JournalEntryPageData{CharacterID: "C", EntryID: "E"}),
	}

	for name, page := range tabs {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, page)
			if !strings.Contains(body, `data-modal-open="/fragment/character/share?character=C"`) {
				t.Errorf("the %s tab has no character share button:\n%s", name, body)
			}
		})
	}
}
