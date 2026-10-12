package pages

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"

	"tabletopper/internal/queries"
)

func TestTheRosterAndTheEditorCarryTheSameAvatarControl(t *testing.T) {
	const id = "01BX5ZZKBKACTAV9WEVGEMMVS4"
	header := testCharacterHeader()
	card := renderToString(t, Character(queries.Character{ID: ulid.MustParse(id), Name: header.Name}))
	editor := renderToString(t, EditCharacter(EditCharacterPageData{CharacterID: id, Header: header}))
	refresh := renderToString(t, CharacterBarValues(id, header))
	for _, want := range []string{
		`hx-post="/characters/` + id + `/avatar"`,
		`hx-target="closest character-avatar"`,
		`hx-swap="outerHTML"`,
		`hx-encoding="multipart/form-data"`,
		`name="avatar"`,
	} {
		for name, markup := range map[string]string{"the roster card": card, "the editor's bar": editor, "the bar refresh": refresh} {
			if !strings.Contains(markup, want) {
				t.Errorf("%s is missing %s", name, want)
			}
		}
	}
	reply := renderToString(t, CharacterAvatarControl(CharacterAvatar{CharacterID: id, Name: header.Name}))
	for name, markup := range map[string]string{"the roster card": card, "the editor's bar": editor, "the bar refresh": refresh} {
		if !strings.Contains(markup, reply) {
			t.Errorf("%s does not render the component the upload replies with", name)
		}
	}
}
func TestTheAvatarControlSaysWhetherItUploadsOrReplaces(t *testing.T) {
	const id = "01BX5ZZKBKACTAV9WEVGEMMVS4"
	empty := renderToString(t, CharacterAvatarControl(CharacterAvatar{CharacterID: id, Name: "Vashti"}))
	if !strings.Contains(empty, `aria-label="Upload character avatar"`) || strings.Contains(empty, "<img") {
		t.Errorf("a character with no portrait is not offered an upload:\n%s", empty)
	}
	linked := renderToString(t, CharacterAvatarControl(CharacterAvatar{CharacterID: id, Name: "Vashti", AvatarID: "01BX5ZZKBKACTAV9WEVGEMMVS5"}))
	if !strings.Contains(linked, `aria-label="Replace character avatar"`) || !strings.Contains(linked, `src="/assets/images/01BX5ZZKBKACTAV9WEVGEMMVS5"`) {
		t.Errorf("a character with a portrait is not shown it and offered a replacement:\n%s", linked)
	}
}
func TestTheAvatarControlDoesNotCarryItsOwnSize(t *testing.T) {
	control := renderToString(t, CharacterAvatarControl(CharacterAvatar{CharacterID: "01BX5ZZKBKACTAV9WEVGEMMVS4", Name: "Vashti"}))
	if !strings.Contains(control, "w-full") {
		t.Error("the control does not fill the box that sizes it")
	}
	for _, size := range []string{"w-14", "w-11"} {
		if strings.Contains(control, size) {
			t.Errorf("the control carries %s; the page that draws it should", size)
		}
	}
}
func TestTheSheetWindowShowsThePortraitWithoutAnUpload(t *testing.T) {
	const id = "01BX5ZZKBKACTAV9WEVGEMMVS4"
	header := testCharacterHeader()
	header.AvatarID = "01BX5ZZKBKACTAV9WEVGEMMVS5"
	window := renderToString(t, CharacterSheetWindow(SheetWindowData{
		Section: SheetSectionMain,
		Sheet:   EditCharacterPageData{CharacterID: id, Header: header},
	}))
	if !strings.Contains(window, `src="/assets/images/`+header.AvatarID+`"`) {
		t.Error("the sheet window lost the portrait")
	}
	if strings.Contains(window, "/avatar\"") || strings.Contains(window, "<character-avatar") {
		t.Error("the sheet window offers an avatar upload")
	}
}
