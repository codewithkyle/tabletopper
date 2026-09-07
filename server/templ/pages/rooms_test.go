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
	}
}

// menuLabels is the headings across the bar, in order.
func menuLabels(data RoomPageData) []string {
	labels := []string{}
	for _, m := range data.Menus() {
		labels = append(labels, m.Label)
	}

	return labels
}

// itemLabels is the lines in one menu, in order. It fails rather than returning
// nothing for a heading that is not there, because a test asking about a menu
// that has been renamed should say so.
func itemLabels(t *testing.T, data RoomPageData, heading string) []string {
	t.Helper()

	for _, m := range data.Menus() {
		if m.Label != heading {
			continue
		}

		labels := []string{}
		for _, item := range m.Items {
			labels = append(labels, item.Label)
		}

		return labels
	}

	t.Fatalf("there is no %q menu; the bar has %v", heading, menuLabels(data))

	return nil
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

// THE BAR IS THE APPLICATION'S SHAPE AND IT IS SETTLED BEFORE THE FEATURES ARE.
// Seven headings, in this order, whoever is looking -- a GM learns where things
// are by muscle memory, and a menu that appears later moves everything under
// it. What varies by role is inside the Room menu and nowhere else.
func TestTheBarHasTheSameSevenMenusForEveryone(t *testing.T) {
	want := []string{"Room", "Tabletop", "Fog", "Initiative", "Window", "View", "Help"}

	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		got := menuLabels(testRoomPage(role))
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s sees %v, want %v", role, got, want)
		}
	}
}

// Every item the bar is meant to carry, pinned by menu. Most of them are
// disabled and that is not what this is about: the point is that the shape is
// here, so building a feature later is wiring an item rather than deciding
// where it lives.
func TestEveryMenuCarriesItsItems(t *testing.T) {
	data := testRoomPage(room.RoleGM)

	for heading, want := range map[string][]string{
		"Tabletop":   {"Settings", "Load image", "Spawn pawns", "Clear tabletop"},
		"Fog":        {"Fill fog", "Clear fog"},
		"Initiative": {"Sync tracker", "Clear tracker"},
		"Window":     {"Monster Manual", "Dice tray"},
		"View":       {"Zoom in", "Zoom out", "100%", "200%", "Toggle fullscreen", "Center tabletop"},
		"Help":       {"Report issue", "Privacy policy", "Terms of service"},
	} {
		got := itemLabels(t, data, heading)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the %s menu is %v, want %v", heading, got, want)
		}
	}
}

// THE ROOM MENU IS THE ONE THAT DIFFERS BY ROLE. The GM owns the room, so they
// get the lock, the code and the close; a player is only in it, so they get a
// way out and nothing else.
func TestTheRoomMenuGivesTheGMTheRoomAndThePlayerTheDoor(t *testing.T) {
	gm := itemLabels(t, testRoomPage(room.RoleGM), "Room")
	want := []string{"Lock room", "Player List", "Copy room code", "Back to rooms", "Close room"}
	if strings.Join(gm, ",") != strings.Join(want, ",") {
		t.Errorf("the GM's Room menu is %v, want %v", gm, want)
	}

	player := itemLabels(t, testRoomPage(room.RolePlayer), "Room")
	want = []string{"Player List", "Leave room"}
	if strings.Join(player, ",") != strings.Join(want, ",") {
		t.Errorf("the player's Room menu is %v, want %v", player, want)
	}
}

// A closed room has no code, so there is nothing to copy, nothing to lock and
// nothing left to close. Reopen takes their place.
func TestAClosedRoomOffersReopenAndNothingElseToDoToIt(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	data.Closed = true
	data.Code = ""

	got := itemLabels(t, data, "Room")
	want := []string{"Reopen room", "Player List", "Back to rooms"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the closed room's menu is %v, want %v", got, want)
	}
}

// The lock is one line and two routes, and the line carries the id the reply
// swaps -- so the item the page draws and the item the route answers with are
// built by the same function and cannot drift.
func TestTheLockItemNamesTheRouteItIsNotIn(t *testing.T) {
	open := markup(t, RoomLockItem(testRoomPage(room.RoleGM)))
	locked := testRoomPage(room.RoleGM)
	locked.Locked = true

	for _, want := range []string{`id="room-lock"`, `hx-post="/rooms/01BX5ZZKBKACTAV9WEVGEMMVT0/lock"`, `hx-target="#room-lock"`, ">Lock room<"} {
		if !strings.Contains(open, want) {
			t.Errorf("an unlocked room's item is missing %s:\n%s", want, open)
		}
	}

	shut := markup(t, RoomLockItem(locked))
	for _, want := range []string{`hx-post="/rooms/01BX5ZZKBKACTAV9WEVGEMMVT0/unlock"`, ">Unlock room<"} {
		if !strings.Contains(shut, want) {
			t.Errorf("a locked room's item is missing %s:\n%s", want, shut)
		}
	}

	// And the page draws the same item, so the first render and the reply match.
	if !strings.Contains(markup(t, Room(testRoomPage(room.RoleGM))), open) {
		t.Error("the page renders its own copy of the lock item")
	}
}

// THE CODE IS THE GM'S. A player who is already in the room has no use for it,
// and it is the one thing on this page that admits somebody else -- so it must
// not be in a player's markup at all, not even in an attribute a script reads.
func TestThePlayersPageNeverCarriesTheRoomCode(t *testing.T) {
	player := markup(t, Room(testRoomPage(room.RolePlayer)))

	if strings.Contains(player, "AB2C") {
		t.Error("the player's page carries the room code")
	}
	for _, forbidden := range []string{"copy-code", "/lock", "/close"} {
		if strings.Contains(player, forbidden) {
			t.Errorf("the player's page carries the GM's %q", forbidden)
		}
	}
	if !strings.Contains(player, "/leave") {
		t.Error("the player's page has no way out of the room")
	}
}

// The two documents open in a second tab, which is the one place in the app
// that is true: following one out of a game in progress should not end it.
func TestTheHelpMenuOpensTheDocumentsInASecondTab(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	for _, path := range []string{"/privacy", "/tos"} {
		if !strings.Contains(page, `href="`+path+`" target="_blank" rel="noopener noreferrer"`) {
			t.Errorf("%s does not open in a second tab", path)
		}
	}
}

// An item whose feature is not built is disabled rather than silently inert. A
// disabled item says "this belongs here and does not work yet"; one that looks
// live and does nothing says "this is broken".
func TestUnbuiltItemsAreDisabledRatherThanInert(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	for _, m := range testRoomPage(room.RoleGM).Menus() {
		for _, item := range m.Items {
			if !item.Disabled {
				continue
			}
			if !strings.Contains(page, "<button type=\"button\" disabled>"+item.Label+"</button>") {
				t.Errorf("%q in the %s menu is not drawn as a disabled control", item.Label, m.Label)
			}
		}
	}

	// menu-disabled is what greys the line the button sits on; without it the
	// row still highlights on hover and reads as clickable.
	if !strings.Contains(page, `class="menu-disabled"`) {
		t.Error("no disabled item carries menu-disabled")
	}
}

// THE SIDE PANEL IS GONE AND STAYS GONE. The table fills everything under the
// bar; the player list is a window behind a menu, initiative is not a column
// beside the map, and there is no chat.
func TestTheRoomIsABarAndATable(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	if !strings.Contains(page, `id="tabletop"`) {
		t.Error("the room has no table region for the canvas to mount on")
	}
	for _, gone := range []string{`id="members"`, `id="initiative-panel"`, `id="messages-panel"`, "<aside"} {
		if strings.Contains(page, gone) {
			t.Errorf("the side panel is back: %s", gone)
		}
	}
	// `table` is a DaisyUI component and Tailwind reads every word in a .templ
	// file as a class-name candidate, attribute values included -- so the wrong
	// id here emits the whole table family and nothing anywhere fails.
	for _, forbidden := range []string{`id="table"`, `id="list"`, `id="status"`, `id="chat"`, `id="stack"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page carries %s, which is a DaisyUI component name", forbidden)
		}
	}
}

// The tool pill is four modes with exactly one pressed, and it is over the
// table rather than in the bar because a pointer mode is switched constantly
// while both hands are busy.
func TestTheToolPillStartsOnExactlyOneTool(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	if got := strings.Count(page, "data-room-tool="); got != len(RoomTools()) {
		t.Errorf("the pill has %d buttons, want %d", got, len(RoomTools()))
	}
	if got := strings.Count(page, `aria-pressed="true"`); got != 1 {
		t.Errorf("%d tools are pressed, want exactly 1", got)
	}
	if !strings.Contains(page, `data-room-tool="`+DefaultRoomTool+`" aria-label="Move" aria-pressed="true"`) {
		t.Errorf("the room does not open on %q", DefaultRoomTool)
	}

	// It floats over the table, so it is inside the region it acts on.
	if strings.Index(page, `id="tabletop"`) > strings.Index(page, "data-room-tools") {
		t.Error("the tool pill is outside the table region")
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
