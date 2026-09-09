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

// menuNamed is one menu of the bar. It fails rather than returning nothing for
// a heading that is not there, because a test asking about a menu that has been
// renamed should say so.
func menuNamed(t *testing.T, data RoomPageData, heading string) RoomMenu {
	t.Helper()

	for _, m := range data.Menus() {
		if m.Label == heading {
			return m
		}
	}

	t.Fatalf("there is no %q menu; the bar has %v", heading, menuLabels(data))

	return RoomMenu{}
}

// itemLabels is the lines in one menu, in order.
func itemLabels(t *testing.T, data RoomPageData, heading string) []string {
	t.Helper()

	labels := []string{}
	for _, item := range menuNamed(t, data, heading).Items {
		labels = append(labels, item.Label)
	}

	return labels
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
		"Tabletop":   {"Layers", "Grid & settings", "Spawn pawns", "Spawn from library", "Clear tabletop"},
		"Fog":        {"Fill fog", "Clear fog"},
		"Initiative": {"Sync tracker", "Clear tracker"},
		"Window":     {"Monster Manual", "Dice tray"},
		"View":       {"Zoom in", "Zoom out", "100%", "200%", "Fit map", "Toggle fullscreen"},
		"Help":       {"Report issue", "Privacy policy", "Terms of service"},
	} {
		got := itemLabels(t, data, heading)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the %s menu is %v, want %v", heading, got, want)
		}
	}
}

// SPAWN PAWNS IS A POST AND NOT A DIALOG, AND IT IS THE GM'S ALONE. Both halves
// of that were built the other way first and both were wrong. The item opened a
// modal that asked nothing the room did not already know -- who is connected and
// which of them have a pawn are room state -- and the player's copy of this menu
// carried a live "Place my pawn" beside four dead lines.
//
// A PLAYER MAY DO NOTHING TO THE TABLE AND THE MENU SAYS SO. That is not merely
// a hidden button: PawnSpawn.Authorize refuses a player outright, so the greyed
// lines here and the socket agree about what is possible.
func TestSpawnPawnsPostsThePartyAndIsTheGMsAlone(t *testing.T) {
	var spawn RoomMenuItem
	for _, item := range menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items {
		if item.Label == "Spawn pawns" {
			spawn = item
		}
	}

	if want := "/rooms/01BX5ZZKBKACTAV9WEVGEMMVT0/pawns/party"; spawn.Post != want {
		t.Errorf("Spawn pawns posts to %q, want %q", spawn.Post, want)
	}
	if spawn.Modal.URL != "" {
		t.Errorf("Spawn pawns opens the modal at %q; it asks nothing", spawn.Modal.URL)
	}
	if spawn.Disabled || spawn.Action != "" || spawn.Window.ID != "" {
		t.Errorf("Spawn pawns is not a plain post: %+v", spawn)
	}

	for _, item := range menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items {
		if !item.Disabled {
			t.Errorf("a player's %q is live; nothing under Tabletop is theirs", item.Label)
		}
	}
}

// CLEAR TABLETOP IS A POST BEHIND A CONFIRMATION, and it is the only item in
// this menu that is either. It empties every floor in one command and there is
// no undo, so the confirm modal has to name what goes -- "Are you sure?" over a
// menu of six items is a question nobody can answer safely.
func TestClearTabletopIsConfirmedAndIsTheGMsAlone(t *testing.T) {
	var clear RoomMenuItem
	for _, item := range menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items {
		if item.Label == "Clear tabletop" {
			clear = item
		}
	}

	if want := "/rooms/01BX5ZZKBKACTAV9WEVGEMMVT0/tabletop/clear"; clear.Post != want {
		t.Errorf("Clear tabletop posts to %q, want %q", clear.Post, want)
	}
	if clear.Disabled {
		t.Error("the GM's Clear tabletop is disabled")
	}
	if !clear.Danger {
		t.Error("Clear tabletop is not drawn as destructive")
	}
	for _, part := range []string{"map", "pawn", "fog", "drawing", "initiative"} {
		if !strings.Contains(clear.Confirm, part) {
			t.Errorf("the confirmation does not mention %s: %q", part, clear.Confirm)
		}
	}

	// It is last, because it is the one item in the menu that undoes the rest.
	items := menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items
	if items[len(items)-1].Label != "Clear tabletop" {
		t.Errorf("Clear tabletop is not the last item: %q", items[len(items)-1].Label)
	}

	for _, item := range menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items {
		if item.Label == "Clear tabletop" && !item.Disabled {
			t.Error("a player can clear the tabletop")
		}
	}
}

// A WINDOW'S TITLE BAR MUST BE ABLE TO SHRINK OR THE WINDOW CANNOT BE CLOSED.
// The bar is a row of a grid whose column is sized auto, so without min-w-0 the
// column's floor is the bar's own min-content width -- and the heading's
// white-space: nowrap makes that the whole title. A long pawn name then pushed
// the three controls past the window's edge, where overflow-hidden cut them off.
func TestAWindowsTitleBarCanShrink(t *testing.T) {
	body := renderToString(t, roomWindowTemplate())

	bar := body[strings.Index(body, "data-window-bar"):]
	bar = bar[:strings.Index(bar, ">")]
	if !strings.Contains(bar, "min-w-0") {
		t.Errorf("the title bar cannot shrink, so a long name pushes the controls out:\n%s", bar)
	}

	if !strings.Contains(body, "truncate") {
		t.Error("the heading does not truncate")
	}
}

// THE CANVAS IS THE RENDERER'S WHOLE FOOTPRINT IN THE MARKUP, and the two
// attributes on it are what render/renderer.ts looks for. touch-none is not
// decoration: without it a browser handles a one finger drag as a scroll and
// the pointer events never arrive, so a tablet gets a table it cannot pan.
func TestAnOpenRoomRendersTheCanvasAndAClosedOneDoesNot(t *testing.T) {
	open := renderToString(t, roomContent(testRoomPage(room.RoleGM)))

	for _, want := range []string{"data-tabletop-canvas", "data-tabletop-unsupported", "touch-none"} {
		if !strings.Contains(open, want) {
			t.Errorf("the table has no %s:\n%s", want, open)
		}
	}

	// The message is rendered by the page and revealed by the client, because
	// server/js is not a Tailwind source and a class name written there would
	// never reach the stylesheet.
	if !strings.Contains(open, "data-tabletop-unsupported hidden") {
		t.Error("the unsupported message is not hidden to begin with")
	}

	// A closed room has no socket, no state and nothing to draw. Putting a
	// canvas there would paint a grid behind the sentence explaining that the
	// room is shut.
	closed := testRoomPage(room.RoleGM)
	closed.Closed = true

	if page := renderToString(t, roomContent(closed)); strings.Contains(page, "data-tabletop-canvas") {
		t.Errorf("a closed room drew a table:\n%s", page)
	}
}

// THE CAMERA ITEMS ARE THE ONE PLACE A LABEL IS NOT THE CONTRACT. Everything
// else in the bar is a URL or a window id, which fails loudly when it is wrong;
// these cross into another bundle as a string in an event, where a typo is a
// menu item that does nothing and reports nothing. So the action and every
// value are pinned here, against public/js/room.js and render/renderer.ts.
func TestEveryCameraItemSendsTheOneViewAction(t *testing.T) {
	want := map[string]string{
		"Zoom in":  "zoom-in",
		"Zoom out": "zoom-out",
		"100%":     "zoom-1",
		"200%":     "zoom-2",
		"Fit map":  "fit",
	}

	seen := 0

	for _, item := range menuNamed(t, testRoomPage(room.RoleGM), "View").Items {
		value, ok := want[item.Label]
		if !ok {
			continue
		}

		seen++
		if item.Action != "view" {
			t.Errorf("%q has action %q, want view", item.Label, item.Action)
		}
		if item.Value != value {
			t.Errorf("%q sends %q, want %q", item.Label, item.Value, value)
		}
		if item.Disabled {
			t.Errorf("%q is disabled", item.Label)
		}
	}

	if seen != len(want) {
		t.Errorf("found %d camera items, want %d", seen, len(want))
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

// The tool pill is five modes with exactly one pressed, and it is over the
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

	// IT OPENS ON SELECT, because picking a pawn out and drawing a box round
	// four of them are the gestures a table is made of -- a room that opened in
	// a mode where none of them worked would have to be switched out of before
	// it could be played.
	if !strings.Contains(page, `data-room-tool="`+DefaultRoomTool+`" aria-label="Select" aria-pressed="true"`) {
		t.Errorf("the room does not open on %q", DefaultRoomTool)
	}

	// It floats over the table, so it is inside the region it acts on.
	if strings.Index(page, `id="tabletop"`) > strings.Index(page, "data-room-tools") {
		t.Error("the tool pill is outside the table region")
	}
}

// WHICH TOOL HANDS THE POINTER TO THE CAMERA IS RENDERED RATHER THAN SPELLED
// AGAIN IN TYPESCRIPT. server/js/room/tools.ts finds it by this attribute,
// because that is the button the space bar borrows -- and a name written out in
// both languages is a space bar that quietly stops working the day this list is
// renamed. Exactly one tool carries it, and nothing about the page says so
// except the list itself.
func TestExactlyOneToolHandsThePointerToTheCamera(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	if got := strings.Count(page, "data-room-tool-pans"); got != 1 {
		t.Fatalf("%d tools pan, want exactly 1", got)
	}
	if !strings.Contains(page, `data-room-tool="`+RoomToolMove+`" data-room-tool-pans`) {
		t.Errorf("the panning tool is not %q:\n%s", RoomToolMove, page)
	}

	pans := 0
	for _, tool := range RoomTools() {
		if tool.Pans {
			pans++
		}
		if tool.Pans && tool.Name == DefaultRoomTool {
			t.Error("the room opens in the mode that takes the pointer away from the table")
		}
	}
	if pans != 1 {
		t.Errorf("%d tools in the list pan, want exactly 1", pans)
	}
}

// AND WHICH ONE IS THE RULER IS RENDERED THE SAME WAY, for the same reason and
// with one difference the client cares about: the space bar borrows the panning
// tool and does not borrow this one, so a measurement survives the map being
// shoved across. Two flags rather than a name per reader is what keeps Fog and
// Draw from needing anything here but a third.
func TestExactlyOneToolIsTheRuler(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))

	if got := strings.Count(page, "data-room-tool-measures"); got != 1 {
		t.Fatalf("%d tools measure, want exactly 1", got)
	}
	if !strings.Contains(page, `data-room-tool="`+RoomToolMeasure+`"`) {
		t.Fatalf("the pill has no %q tool:\n%s", RoomToolMeasure, page)
	}

	measures := 0
	for _, tool := range RoomTools() {
		if tool.Measures {
			measures++
		}
		if tool.Measures && tool.Name != RoomToolMeasure {
			t.Errorf("%q measures as well as %q", tool.Name, RoomToolMeasure)
		}

		// NO TOOL DOES BOTH. panning is asked of the lit button and measuring
		// of the chosen one, and a button that answered yes to both would put
		// the table in two modes the moment the space bar went down.
		if tool.Pans && tool.Measures {
			t.Errorf("%q both pans and measures", tool.Name)
		}
	}
	if measures != 1 {
		t.Errorf("%d tools in the list measure, want exactly 1", measures)
	}
}

// The floors menu swaps the layer the PLAYERS are shown, which is the GM's
// alone -- so a player's page renders neither the button nor the list, and the
// client finds nothing to mount rather than a control it has to hide.
func TestTheFloorMenuIsTheGMsAlone(t *testing.T) {
	gm := markup(t, Room(testRoomPage(room.RoleGM)))
	player := markup(t, Room(testRoomPage(room.RolePlayer)))

	for _, needed := range []string{"data-layer-tool", "data-layer-menu", "data-layer-menu-template", "data-layer-menu-choice"} {
		if !strings.Contains(gm, needed) {
			t.Errorf("the GM has no %s to swap floors with", needed)
		}
		if strings.Contains(player, needed) {
			t.Errorf("a player was rendered %s", needed)
		}
	}

	// It is placed against the table rather than inside the pill, which is
	// positioned and z-indexed and would trap it under any window sitting over
	// that corner.
	if strings.Index(gm, "data-layer-menu") < strings.Index(gm, "data-room-tools") {
		t.Error("the floors menu is rendered before the pill it belongs to")
	}
	if strings.Index(gm, `id="tabletop"`) > strings.Index(gm, "data-layer-menu") {
		t.Error("the floors menu is outside the table region")
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
