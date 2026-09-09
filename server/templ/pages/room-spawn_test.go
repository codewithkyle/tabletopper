package pages

import (
	"strings"
	"testing"
)

// A TOKEN IS AN OBJECT AND THE DIALOG ASKS NOTHING ABOUT IT. It used to carry a
// Creature-or-Object switch, a creature-size select and two cell-count inputs;
// all four are gone, because a token is a picture of a thing on the table and
// the assets row already records how big the picture is.
func TestTheTokenHalfAsksNothingAboutTheToken(t *testing.T) {
	body := renderToString(t, RoomSpawn(RoomSpawnData{
		RoomID: "01BX5ZZKBKACTAV9WEVGEMMVT0",
		Kind:   RoomSpawnTokens,
		Tokens: []RoomSpawnToken{{ID: "01TOKEN", Name: "Wagon", Image: "/assets/images/01TOKEN", Width: 300, Height: 100}},
	}))

	for _, gone := range []string{"data-spawn-as", "data-spawn-size", "data-spawn-creature", "data-spawn-object"} {
		if strings.Contains(body, gone) {
			t.Errorf("the dialog still carries %s:\n%s", gone, body)
		}
	}

	// The one question it does still ask, because the answer is nowhere else:
	// is the GM putting this down in front of the party or setting up the next
	// room while they talk.
	if !strings.Contains(body, "data-spawn-shown") {
		t.Error("the dialog does not ask whether players see it")
	}
}

// THE CARD CARRIES THE PICTURE'S PIXELS so the ghost following the pointer is
// the size of the thing about to be placed. The pawn's actual size is read from
// the same row again when the spawn is resolved, so these are a preview and not
// an input.
func TestATokenCardCarriesItsPicturesSize(t *testing.T) {
	body := renderToString(t, RoomSpawnList(RoomSpawnData{
		Kind:   RoomSpawnTokens,
		Tokens: []RoomSpawnToken{{ID: "01TOKEN", Name: "Wagon", Image: "/x", Width: 300, Height: 100}},
	}))

	for _, want := range []string{`data-spawn-width="300"`, `data-spawn-height="100"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the card has no %s:\n%s", want, body)
		}
	}
}

// A ROW WRITTEN BEFORE THE SIZE COLUMNS EXISTED LEAVES THE ATTRIBUTES EMPTY
// rather than printing a zero, which is what lets the client tell "not recorded"
// from "nothing wide" and fall back to one cell -- the same fallback the hub
// applies to the pawn itself.
func TestATokenWithNoRecordedSizeSaysSoWithSilence(t *testing.T) {
	body := renderToString(t, RoomSpawnList(RoomSpawnData{
		Kind:   RoomSpawnTokens,
		Tokens: []RoomSpawnToken{{ID: "01TOKEN", Name: "Wagon", Image: "/x"}},
	}))

	if strings.Contains(body, `data-spawn-width="0"`) || strings.Contains(body, `data-spawn-height="0"`) {
		t.Errorf("an unrecorded size was printed as a zero:\n%s", body)
	}
}

// THE NPC FORM ASKS THE FOUR THINGS NOTHING ELSE KNOWS, which is what separates
// it from the other two walls: a monster's numbers are in the manual and a
// token has none, but a face out of the avatar library has neither a row nor an
// excuse. The hooks are what the room bundle reads them back through.
func TestTheNPCFormAsksForAStatLine(t *testing.T) {
	body := renderToString(t, RoomSpawnNPC(RoomSpawnNPCData{
		RoomID: "01BX5ZZKBKACTAV9WEVGEMMVT0",
		Avatar: RoomSpawnAvatar{ID: "01AVATAR", Name: "Innkeeper", Image: "/assets/images/01AVATAR"},
	}))

	for _, want := range []string{"data-npc-name", "data-npc-hp", "data-npc-maxhp", "data-npc-ac", "data-npc-size"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form has no %s:\n%s", want, body)
		}
	}

	// The Place button is the pick, and it carries the face rather than the
	// numbers: the numbers are read off the controls beside it.
	for _, want := range []string{`data-spawn-source="npc"`, `data-spawn-id="01AVATAR"`, "data-spawn-shown"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form has no %s:\n%s", want, body)
		}
	}
}

// THE NAME BOX IS REQUIRED AND STARTS EMPTY. The picture's own name is what the
// GM calls the file -- "bearded man" -- and the party meets Aldric, so
// prefilling it would make pressing Place without reading it the easy path.
func TestTheNPCFormAsksForANameRatherThanBorrowingOne(t *testing.T) {
	body := renderToString(t, RoomSpawnNPC(RoomSpawnNPCData{
		RoomID: "01ROOM",
		Avatar: RoomSpawnAvatar{ID: "01AVATAR", Name: "bearded man"},
	}))

	if !strings.Contains(body, `data-npc-name`) || !strings.Contains(body, `required`) {
		t.Errorf("the name box is missing or optional:\n%s", body)
	}
	if strings.Contains(body, `value="bearded man"`) {
		t.Errorf("the name box was prefilled with the picture's own name:\n%s", body)
	}
}

// BACK RETURNS TO THE WALL THAT WAS BEING LOOKED AT and not to all of it. The
// form has no search box for htmx to include, so the term is baked into the one
// URL that needs it.
func TestBackFromTheNPCFormKeepsTheSearch(t *testing.T) {
	data := RoomSpawnNPCData{RoomID: "01ROOM", Query: "inn keeper"}

	if got, want := data.BackPath(), "/fragment/room/spawn?room=01ROOM&kind=npcs&q=inn+keeper"; got != want {
		t.Errorf("BackPath() = %q, want %q", got, want)
	}

	if got, want := (RoomSpawnNPCData{RoomID: "01ROOM"}).BackPath(), "/fragment/room/spawn?room=01ROOM&kind=npcs"; got != want {
		t.Errorf("an unfiltered wall answered %q, want %q", got, want)
	}
}

// A FACE IS PICKED IN TWO STEPS AND THE CARD IS THE FIRST OF THEM, so an avatar
// card must not arm anything on its own: it fetches the form that asks.
func TestAnAvatarCardOpensTheFormRatherThanArming(t *testing.T) {
	body := renderToString(t, RoomSpawnList(RoomSpawnData{
		RoomID:  "01ROOM",
		Kind:    RoomSpawnNPCs,
		Avatars: []RoomSpawnAvatar{{ID: "01AVATAR", Name: "Innkeeper", Image: "/assets/images/01AVATAR"}},
	}))

	if strings.Contains(body, "data-spawn-pick") {
		t.Errorf("an avatar card arms the canvas without asking for a stat line:\n%s", body)
	}
	if !strings.Contains(body, "/fragment/room/spawn-npc?room=01ROOM&amp;asset=01AVATAR") {
		t.Errorf("an avatar card does not open the form:\n%s", body)
	}
}
