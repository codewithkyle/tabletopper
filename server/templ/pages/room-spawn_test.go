package pages
import (
	"strings"
	"testing"
)
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
	if !strings.Contains(body, "data-spawn-shown") {
		t.Error("the dialog does not ask whether players see it")
	}
}
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
func TestATokenWithNoRecordedSizeSaysSoWithSilence(t *testing.T) {
	body := renderToString(t, RoomSpawnList(RoomSpawnData{
		Kind:   RoomSpawnTokens,
		Tokens: []RoomSpawnToken{{ID: "01TOKEN", Name: "Wagon", Image: "/x"}},
	}))
	if strings.Contains(body, `data-spawn-width="0"`) || strings.Contains(body, `data-spawn-height="0"`) {
		t.Errorf("an unrecorded size was printed as a zero:\n%s", body)
	}
}
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
	for _, want := range []string{`data-spawn-source="npc"`, `data-spawn-id="01AVATAR"`, "data-spawn-shown"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form has no %s:\n%s", want, body)
		}
	}
}
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
func TestBackFromTheNPCFormKeepsTheSearch(t *testing.T) {
	data := RoomSpawnNPCData{RoomID: "01ROOM", Query: "inn keeper"}
	if got, want := data.BackPath(), "/fragment/room/spawn?room=01ROOM&kind=npcs&q=inn+keeper"; got != want {
		t.Errorf("BackPath() = %q, want %q", got, want)
	}
	if got, want := (RoomSpawnNPCData{RoomID: "01ROOM"}).BackPath(), "/fragment/room/spawn?room=01ROOM&kind=npcs"; got != want {
		t.Errorf("an unfiltered wall answered %q, want %q", got, want)
	}
}
func TestAnAvatarCardOpensTheFormRatherThanArming(t *testing.T) {
	body := renderToString(t, RoomSpawnList(RoomSpawnData{
		RoomID:  "01ROOM",
		Kind:    RoomSpawnNPCs,
		Avatars: []RoomSpawnAvatar{{RoomID: "01ROOM", ID: "01AVATAR", Name: "Innkeeper", Image: "/assets/images/01AVATAR"}},
	}))
	if strings.Contains(body, "data-spawn-pick") {
		t.Errorf("an avatar card arms the canvas without asking for a stat line:\n%s", body)
	}
	if !strings.Contains(body, "/fragment/room/spawn-npc?room=01ROOM&amp;asset=01AVATAR") {
		t.Errorf("an avatar card does not open the form:\n%s", body)
	}
}
func TestEachWallAddsItsOwnKind(t *testing.T) {
	cases := map[string]struct {
		kind string
		want string
		post string
	}{
		"monsters": {kind: RoomSpawnMonsters, want: "Create monster", post: "/fragment/room/spawn-monster?room=01ROOM"},
		"tokens":   {kind: RoomSpawnTokens, want: "Upload token", post: "/rooms/01ROOM/spawn/tokens"},
		"npcs":     {kind: RoomSpawnNPCs, want: "Upload avatar", post: "/rooms/01ROOM/spawn/avatars"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, RoomSpawn(RoomSpawnData{RoomID: "01ROOM", Kind: tc.kind}))
			if !strings.Contains(body, tc.want) {
				t.Errorf("the %s wall does not offer %q:\n%s", name, tc.want, body)
			}
			if !strings.Contains(body, tc.post) {
				t.Errorf("the %s wall does not reach %q:\n%s", name, tc.post, body)
			}
			for other, unwanted := range map[string]string{"monsters": "Create monster", "tokens": "Upload token", "npcs": "Upload avatar"} {
				if other != name && strings.Contains(body, unwanted) {
					t.Errorf("the %s wall also offers %q", name, unwanted)
				}
			}
		})
	}
}
func TestAnUploadedCardIsThePickCard(t *testing.T) {
	token := renderToString(t, RoomSpawnTokenCard(RoomSpawnToken{ID: "01TOKEN", Name: "Cart", Image: "/x", Width: 300, Height: 100}))
	if !strings.Contains(token, "data-spawn-pick") || !strings.Contains(token, `data-spawn-source="token"`) {
		t.Errorf("an uploaded token does not arm the canvas:\n%s", token)
	}
	face := renderToString(t, RoomSpawnAvatarCard(RoomSpawnAvatar{RoomID: "01ROOM", ID: "01AVATAR", Name: "Aldric"}))
	if !strings.Contains(face, "/fragment/room/spawn-npc?room=01ROOM&amp;asset=01AVATAR") {
		t.Errorf("an uploaded face does not open the form:\n%s", face)
	}
}
func TestTheQuickMonsterFormPostsToTheManual(t *testing.T) {
	body := renderToString(t, RoomSpawnMonster(RoomSpawnMonsterData{RoomID: "01ROOM"}))
	for _, want := range []string{`name="image"`, `name="name"`, `name="hp"`, `name="ac"`, `name="size"`, "/rooms/01ROOM/spawn/monsters"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form has no %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "data-spawn-pick") {
		t.Error("the quick-create form arms the canvas; it writes a row and comes back to the wall")
	}
	if strings.Contains(body, "data-spawn-shown") {
		t.Error("the quick-create form asks whether players see it, and it places nothing")
	}
}
