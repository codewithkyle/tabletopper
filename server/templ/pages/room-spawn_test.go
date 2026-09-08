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
