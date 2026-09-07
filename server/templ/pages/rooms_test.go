package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)

// testRoomPage is a GM's view of an open room, for the renders that want a page
// with something on it.
func testRoomPage(role room.Role) RoomPageData {
	return RoomPageData{
		ID:   "01BX5ZZKBKACTAV9WEVGEMMVT0",
		Name: "Curse of Strahd",
		Code: "AB2C",
		Role: role,
		Members: RoomMembersData{
			RoomID:  "01BX5ZZKBKACTAV9WEVGEMMVT0",
			Members: []RoomMember{{Name: "Ireena", AvatarURL: "/images/default-avatar.webp", Character: "Vex"}},
		},
	}
}

// THE 422 ROUTE IS WHAT MAKES A REJECTION VISIBLE. Every 4xx is in the noSwap
// list in base.templ, so without an hx-status pointing the response at the
// error block a refused submission would swap nothing and the form would look
// like it had done nothing at all. The four attributes have to agree on one id,
// and a target that has drifted from its block fails silently.
func TestTheNewRoomDialogRoutesItsRejectionToItsErrorBlock(t *testing.T) {
	dialog := markup(t, NewRoomFragment())

	block := "#errors-" + NewRoomPanel
	for _, want := range []string{
		`hx-post="/rooms"`,
		`hx-target="` + block + `"`,
		`hx-swap="outerHTML"`,
		`hx-status:422="target:` + block + `,swap:outerHTML"`,
		`id="errors-` + NewRoomPanel + `"`,
	} {
		if !strings.Contains(dialog, want) {
			t.Errorf("the dialog is missing %s:\n%s", want, dialog)
		}
	}

	// Close first and the affirmative action second, and Close dispatches the
	// event rather than being a <form method="dialog"> -- forms do not nest,
	// and this one is inside the create form.
	if !strings.Contains(dialog, "modal:close") || !strings.Contains(dialog, ">Close<") {
		t.Errorf("the dialog has no way out of it:\n%s", dialog)
	}
	if strings.Index(dialog, ">Close<") > strings.Index(dialog, ">Create room<") {
		t.Error("Close comes after the affirmative action")
	}
}

// The join form takes the same route, plus one for the rate limit. 429 is not
// 422, and without its own hx-status the noSwap list would swallow the message
// that says to wait a minute.
func TestTheJoinFormRoutesBothOfItsRejections(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{}))

	block := "#errors-" + JoinRoomPanel
	for _, want := range []string{
		`hx-post="/rooms/join"`,
		`hx-target="` + block + `"`,
		`hx-swap="outerHTML"`,
		`hx-status:422="target:` + block + `,swap:outerHTML"`,
		`hx-status:429="target:` + block + `,swap:outerHTML"`,
		`id="errors-` + JoinRoomPanel + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the join form is missing %s", want)
		}
	}
}

// The code field is bounded by the generator's own length, so the two cannot
// drift: a field that let five characters through would send a code no room can
// have to a handler that refuses it.
func TestTheJoinFieldIsAsLongAsACode(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{}))

	if !strings.Contains(page, `maxlength="4"`) || room.CodeLength != 4 {
		t.Errorf("the code field does not carry the code's own length (%d)", room.CodeLength)
	}
}

// A pasted /rooms/join/{code} link prefills the field and joins nothing: the
// form is still there to submit.
func TestAPrefilledJoinPageStillHasToBeSubmitted(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Code: "AB2C"}))

	if !strings.Contains(page, `value="AB2C"`) {
		t.Error("the code was not prefilled")
	}
	if !strings.Contains(page, `<button type="submit"`) {
		t.Error("the prefilled page has no submit button, so the code would never be sent")
	}
}

// The picker always offers "No character", because a player may join before
// they have made one -- and it is the first option, so it is what an untouched
// form submits.
func TestTheJoinPickerAlwaysOffersNoCharacter(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{
		Characters: []JoinCharacterOption{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Vex"}},
	}))

	if !strings.Contains(page, `<option value="">No character</option>`) {
		t.Errorf("the picker does not offer joining without a character:\n%s", page)
	}
	if strings.Index(page, "No character") > strings.Index(page, "Vex") {
		t.Error("a character is offered above No character, so an untouched form would submit it")
	}
}

// THE GM'S CONTROLS ARE THE ROLE'S AND NOT THE PAGE'S. A player holding the
// same URL renders the same shell without them, and without the code -- which
// is the one thing on this page that admits somebody else.
func TestTheRoomPageDrawsTheGMControlsForTheGMOnly(t *testing.T) {
	gm := markup(t, Room(testRoomPage(room.RoleGM)))
	player := markup(t, Room(testRoomPage(room.RolePlayer)))

	for _, want := range []string{"/lock", "/close", "AB2C", `id="room-code"`} {
		if !strings.Contains(gm, want) {
			t.Errorf("the GM's page is missing %q", want)
		}
		if strings.Contains(player, want) {
			t.Errorf("the player's page carries the GM's %q", want)
		}
	}

	if !strings.Contains(player, "/leave") {
		t.Error("the player's page has no way out of the room")
	}
	if strings.Contains(gm, "/leave") {
		t.Error("the GM's page offers Leave, which is a member's control")
	}
}

// A closed room offers Reopen instead of the code, the lock and Close -- and
// only the GM ever sees it, because a player is turned out when it closes.
func TestAClosedRoomOffersReopenInsteadOfTheTableControls(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	data.Closed = true
	data.Code = ""

	page := markup(t, Room(data))

	if !strings.Contains(page, "/open") {
		t.Error("the closed room offers no way to reopen it")
	}
	for _, forbidden := range []string{"/close", "/lock", `id="room-code"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the closed room still carries %q", forbidden)
		}
	}
}

// THE TABLE REGION IS id="tabletop" AND NOT id="table". `table` is a DaisyUI
// component and Tailwind reads every word in a .templ file as a class-name
// candidate, attribute values included -- so the wrong id here emits the whole
// table family into the stylesheet and nothing anywhere fails.
func TestTheRoomPageDoesNotNameADaisyUIComponent(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	if !strings.Contains(page, `id="tabletop"`) {
		t.Error("the room has no table region for the canvas to mount on")
	}
	for _, forbidden := range []string{`id="table"`, `id="list"`, `id="status"`, `id="chat"`, `id="stack"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page carries %s, which is a DaisyUI component name", forbidden)
		}
	}
}

// The panel the page draws and the panel the fragment answers with are the same
// component, so a refresh cannot render something the page never did.
func TestTheMembersPanelIsOneComponent(t *testing.T) {
	data := testRoomPage(room.RoleGM)

	page := markup(t, Room(data))
	fragment := markup(t, RoomMembersFragment(data.Members))

	if !strings.Contains(page, fragment) {
		t.Error("the page renders its own copy of the members panel")
	}
	if !strings.Contains(fragment, `hx-get="/fragment/room/members?room=`+data.ID+`"`) {
		t.Errorf("the panel cannot refetch itself:\n%s", fragment)
	}
}

// A room with nobody in it says so rather than rendering an empty box.
func TestAnEmptyRoomSaysNobodyHasJoined(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	data.Members.Members = nil

	if !strings.Contains(markup(t, Room(data)), "Nobody has joined yet.") {
		t.Error("an empty members panel says nothing")
	}
}

// A card shows the code when the room is open and says Closed when it is not.
// It never prints the code a closed room used to have: that went back into
// circulation and may belong to somebody else's table now.
func TestARoomCardShowsTheCodeOnlyWhileItIsOpen(t *testing.T) {
	open := markup(t, roomCard(RoomSummary{ID: "01BX5ZZKBKACTAV9WEVGEMMVT0", Name: "Curse of Strahd", Code: "AB2C"}))
	closed := markup(t, roomCard(RoomSummary{ID: "01BX5ZZKBKACTAV9WEVGEMMVT0", Name: "Curse of Strahd", Closed: true}))

	if !strings.Contains(open, "AB2C") {
		t.Error("an open room's card does not show its code")
	}
	if strings.Contains(open, "/open") {
		t.Error("an open room's card offers Reopen")
	}
	if !strings.Contains(closed, "/open") {
		t.Error("a closed room's card offers no way to reopen it")
	}

	// Delete is behind the confirm modal on both, because it takes the room and
	// everybody in it.
	for _, card := range []string{open, closed} {
		if !strings.Contains(card, "hx-confirm=") {
			t.Errorf("a room can be deleted without confirming it:\n%s", card)
		}
	}
}
