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

// oneCharacter is a roster with something in it, which every test of the join
// FORM needs: a page rendered for an account with no characters has no form on
// it at all, which is the point of the two tests at the bottom of this group.
func oneCharacter() []JoinCharacterOption {
	return []JoinCharacterOption{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Vex"}}
}

// The join form takes the same route, plus one for the rate limit. 429 is not
// 422, and without its own hx-status the noSwap list would swallow the message
// that says to wait a minute.
func TestTheJoinFormRoutesBothOfItsRejections(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))

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
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))

	if !strings.Contains(page, `maxlength="4"`) || room.CodeLength != 4 {
		t.Errorf("the code field does not carry the code's own length (%d)", room.CodeLength)
	}
}

// THE FIELD IS THE OTP COMPONENT AND ONE BOX IS ONE CHARACTER. DaisyUI draws
// the boxes from the spans, and the input is what is actually typed into, so a
// field with five boxes and a maxlength of four is a box that can never be
// filled -- and four boxes with no maxlength is a code that runs past the last
// one. Both halves are asserted here because nothing else can notice.
func TestTheJoinFieldHasOneBoxPerCharacter(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))

	otp := strings.Index(page, `class="otp`)
	if otp < 0 {
		t.Fatalf("the code field is not the otp component:\n%s", page)
	}

	end := strings.Index(page[otp:], "</label>")
	if end < 0 {
		t.Fatalf("the otp label is not closed:\n%s", page)
	}

	if boxes := strings.Count(page[otp:otp+end], "<span></span>"); boxes != room.CodeLength {
		t.Errorf("the field draws %d boxes, want %d", boxes, room.CodeLength)
	}
}

// The pattern comes off the alphabet rather than out of the markup, so a letter
// left out of codes is a letter the field refuses without anybody remembering
// to change two places.
func TestTheJoinFieldRefusesTheLettersCodesDoNotUse(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))

	if !strings.Contains(page, `pattern="`+room.CodePattern()+`"`) {
		t.Errorf("the code field does not carry the code's own pattern (%s)", room.CodePattern())
	}
	for _, banned := range []string{"I", "L", "O", "0", "1"} {
		if strings.Contains(room.CodePattern(), banned) {
			t.Errorf("the pattern accepts %q, which is not in the alphabet", banned)
		}
	}
}

// A pasted /rooms/join/{code} link prefills the field and joins nothing: the
// form is still there to submit.
func TestAPrefilledJoinPageStillHasToBeSubmitted(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Code: "AB2C", Characters: oneCharacter()}))

	if !strings.Contains(page, `value="AB2C"`) {
		t.Error("the code was not prefilled")
	}
	if !strings.Contains(page, `<button type="submit"`) {
		t.Error("the prefilled page has no submit button, so the code would never be sent")
	}
}

// THE PICKER CANNOT BE LEFT UNANSWERED. Its first option carries the empty
// value and is disabled, so an untouched form submits nothing the handler will
// take -- and `required` is what stops it being submitted at all. Neither is
// the check: JoinRoomForm refuses an empty value, because a form is markup.
func TestTheJoinPickerCannotBeLeftUnanswered(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))

	if !strings.Contains(page, `<option value="" disabled selected>`) {
		t.Errorf("the picker's placeholder is not a disabled empty option:\n%s", page)
	}
	if strings.Contains(page, "No character") {
		t.Error("the picker still offers joining without a character")
	}
	if !strings.Contains(page, `name="character"`) || !strings.Contains(page, `class="select validator peer w-full" required`) {
		t.Errorf("the picker is not required:\n%s", page)
	}
}

// AN ACCOUNT WITH NO CHARACTERS GETS NO FORM. A picker with nothing in it and a
// button that cannot work is a page that looks broken; the honest version says
// what is missing and links to where it is made.
func TestAJoinPageWithNoCharactersOffersNoForm(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Code: "AB2C"}))

	if strings.Contains(page, `hx-post="/rooms/join"`) {
		t.Errorf("the page offers a form it cannot complete:\n%s", page)
	}
	if !strings.Contains(page, `href="/characters"`) {
		t.Errorf("the page does not say where a character is made:\n%s", page)
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

// The player window sorts the GM to the top and everybody else by name, and it
// has to do it the same way every time: this list is refetched on every join,
// every reconnect and every disconnect, and one that reordered itself each time
// would be one nobody could read.
func TestThePlayerWindowPutsTheGMFirstAndTheRestByName(t *testing.T) {
	got := SortRoomMembers([]RoomMember{
		{Name: "ari"},
		{Name: "Sam"},
		{Name: "Kyle", IsGM: true},
		{Name: "Blake"},
	})

	want := []string{"Kyle", "ari", "Blake", "Sam"}
	for i, member := range got {
		if member.Name != want[i] {
			t.Fatalf("order = %v, want %v", names(got), want)
		}
	}
}

func names(members []RoomMember) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Name)
	}

	return out
}

// The Player List opens a window, for the GM and for a player alike -- who is
// at the table is not a secret from the table.
func TestThePlayerListItemOpensAWindow(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		var item RoomMenuItem
		for _, candidate := range testRoomPage(role).roomMenu().Items {
			if candidate.Label == "Player List" {
				item = candidate
			}
		}

		if item.Disabled {
			t.Errorf("%s: Player List is still disabled", role)
		}
		if item.Window.ID != "players" {
			t.Errorf("%s: Player List opens window %q, want %q", role, item.Window.ID, "players")
		}
		if item.Window.Title == "" {
			t.Errorf("%s: the window has no title to put in its bar", role)
		}
		// THE URL HAS TO BE A FRAGMENT. The client refuses anything else, for
		// the reason the content modal does: a page URL swapped into a window
		// is a whole document inside a panel, and it reads as a styling bug.
		if !strings.HasPrefix(item.Window.URL, "/fragment/") {
			t.Errorf("%s: the window loads %q, which is not a fragment", role, item.Window.URL)
		}
	}
}

// The trigger is three data attributes and the client reads all three. An item
// that lost one would open nothing and log, which is a bug nobody sees until
// they click the menu.
func TestAWindowItemRendersItsTrigger(t *testing.T) {
	markup := renderToString(t, RoomLockItem(testRoomPage(room.RoleGM)))
	if strings.Contains(markup, "data-window") {
		t.Fatal("the lock item, which is not a window, rendered window attributes")
	}

	data := testRoomPage(room.RoleGM)
	var item RoomMenuItem
	for _, candidate := range data.roomMenu().Items {
		if candidate.Label == "Player List" {
			item = candidate
		}
	}

	markup = renderToString(t, roomBarItem(item))
	for _, want := range []string{
		`data-window="players"`,
		`data-window-title="Players"`,
		`data-window-url="/fragment/room/members?room=` + data.ID + `"`,
		`data-window-width="260"`,
		`data-window-height="260"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the trigger is missing %s\n%s", want, markup)
		}
	}
}

// An unset size renders no attribute at all, so the client's own default is
// what applies rather than a zero.
func TestAWindowWithNoSizeRendersNoSizeAttributes(t *testing.T) {
	markup := renderToString(t, roomBarItem(RoomMenuItem{
		Label:  "Dice tray",
		Window: RoomWindow{ID: "dice", Title: "Dice tray", URL: "/fragment/room/dice"},
	}))

	for _, unwanted := range []string{"data-window-width", "data-window-height"} {
		if strings.Contains(markup, unwanted) {
			t.Errorf("an unsized window rendered %s", unwanted)
		}
	}
}

// testRoomIDText is any well-formed ULID: these tests render markup and never
// parse it back, so what matters is that the id shows up in the URLs the
// buttons carry.
const testRoomIDText = "01BX5ZZKBKACTAV9WEVGEMMVT0"

// membersFor is a room with the GM and one player in it, drawn for a viewer
// who either may or may not remove them.
func membersFor(canKick bool) RoomMembersData {
	return RoomMembersData{
		RoomID:  testRoomIDText,
		Live:    true,
		CanKick: canKick,
		Members: SortRoomMembers([]RoomMember{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVT2", Name: MemberName(false, "Ilyana", "ari"), Username: "ari", Connected: true},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVT3", Name: MemberName(true, "", "kyle"), Username: "kyle", IsGM: true, Connected: true},
		}),
	}
}

// THE REMOVE BUTTON IS THE GM'S AND IT IS NEVER ON THEIR OWN ROW. A GM cannot
// remove themselves -- PlayerKick.Authorize refuses it, because a room with
// nobody who can unlock or close it has to be abandoned -- so drawing the
// button there would be an affordance for a refusal.
func TestTheRemoveButtonIsTheGMsAndSkipsTheirOwnRow(t *testing.T) {
	data := membersFor(true)
	page := renderToString(t, RoomMembers(data))

	player := data.Members[1]
	if !strings.Contains(page, `hx-post="`+data.KickPath(player)+`"`) {
		t.Errorf("no remove button for the player:\n%s", page)
	}
	if got := strings.Count(page, "hx-post="); got != 1 {
		t.Errorf("%d remove buttons for a room of two, want 1 (not the GM's own row)", got)
	}

	// The destructive gate is hx-confirm, which is the app's only one, and it
	// names the person rather than asking "Are you sure?" over a list.
	if !strings.Contains(page, `hx-confirm="`+data.KickPrompt(player)+`"`) {
		t.Errorf("the remove button has no confirm:\n%s", page)
	}
	if !strings.Contains(page, "Ilyana") {
		t.Error("the confirm does not name the person it is about")
	}

	// The reply is the list, so the button has to say where it goes.
	if !strings.Contains(page, `hx-target="#room-members"`) {
		t.Errorf("the remove button does not target the list it replaces:\n%s", page)
	}
}

// A PLAYER SEES NO BUTTON, and that is a courtesy rather than the rule: the
// refusal is PlayerKick.Authorize, which runs against a role derived from the
// rooms row on every post whether or not a button was drawn.
func TestAPlayerSeesNoRemoveButton(t *testing.T) {
	page := renderToString(t, RoomMembers(membersFor(false)))

	if strings.Contains(page, "hx-post=") {
		t.Errorf("the player list offers a player a remove button:\n%s", page)
	}
}
