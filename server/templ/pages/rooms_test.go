package pages

import (
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/prefs"
	"tabletopper/internal/room"
)

func testRoomPage(role room.Role) RoomPageData {
	return RoomPageData{
		ID:         "01BX5ZZKBKACTAV9WEVGEMMVT0",
		Name:       "Curse of Strahd",
		Code:       "AB2C",
		Role:       role,
		PingVolume: prefs.PingVolumeMax,
	}
}
func menuLabels(data RoomPageData) []string {
	labels := []string{}
	for _, m := range data.Menus() {
		labels = append(labels, m.Label)
	}
	return labels
}
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
func itemLabels(t *testing.T, data RoomPageData, heading string) []string {
	t.Helper()
	labels := []string{}
	for _, item := range menuNamed(t, data, heading).Items {
		labels = append(labels, item.Label)
	}
	return labels
}
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
	if !strings.Contains(dialog, "modal:close") || !strings.Contains(dialog, ">Close<") {
		t.Errorf("the dialog has no way out of it:\n%s", dialog)
	}
	if strings.Index(dialog, ">Close<") > strings.Index(dialog, ">Create room<") {
		t.Error("Close comes after the affirmative action")
	}
}
func oneCharacter() []JoinCharacterOption {
	return []JoinCharacterOption{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Vex"}}
}
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
func TestTheJoinFieldIsAsLongAsACode(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Characters: oneCharacter()}))
	if !strings.Contains(page, `maxlength="4"`) || room.CodeLength != 4 {
		t.Errorf("the code field does not carry the code's own length (%d)", room.CodeLength)
	}
}
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
func TestAPrefilledJoinPageStillHasToBeSubmitted(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Code: "AB2C", Characters: oneCharacter()}))
	if !strings.Contains(page, `value="AB2C"`) {
		t.Error("the code was not prefilled")
	}
	if !strings.Contains(page, `<button type="submit"`) {
		t.Error("the prefilled page has no submit button, so the code would never be sent")
	}
}
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
func TestAJoinPageWithNoCharactersOffersNoForm(t *testing.T) {
	page := renderToString(t, JoinRoom(JoinRoomPageData{Code: "AB2C"}))
	if strings.Contains(page, `hx-post="/rooms/join"`) {
		t.Errorf("the page offers a form it cannot complete:\n%s", page)
	}
	if !strings.Contains(page, `href="/characters"`) {
		t.Errorf("the page does not say where a character is made:\n%s", page)
	}
}
func TestTheBarGivesEachRoleTheHeadingsTheyCanAct(t *testing.T) {
	for role, want := range map[room.Role][]string{
		room.RoleGM:     {"Room", "Tabletop", "Fog", "Initiative", "Tools", "View", "Help"},
		room.RolePlayer: {"Room", "Tabletop", "Character", "Tools", "View", "Help"},
	} {
		got := menuLabels(testRoomPage(role))
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s sees %v, want %v", role, got, want)
		}
	}
}
func TestTheCharacterMenuIsThePlayersAlone(t *testing.T) {
	got := itemLabels(t, testRoomPage(room.RolePlayer), "Character")
	want := []string{"Character sheet", "Journal"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the Character menu is %v, want %v", got, want)
	}
	for _, m := range testRoomPage(room.RoleGM).Menus() {
		if m.Label == "Character" {
			t.Error("the GM's bar carries a Character menu; they are running the table, not playing at it")
		}
	}
}
func TestTheMonsterManualIsTheGMsAndTheDiceTrayIsEverybodys(t *testing.T) {
	gm := itemLabels(t, testRoomPage(room.RoleGM), "Tools")
	want := []string{"Monster Manual", "Dice tray", "Music"}
	if strings.Join(gm, ",") != strings.Join(want, ",") {
		t.Errorf("the GM's Tools menu is %v, want %v", gm, want)
	}
	player := itemLabels(t, testRoomPage(room.RolePlayer), "Tools")
	want = []string{"Dice tray", "Music"}
	if strings.Join(player, ",") != strings.Join(want, ",") {
		t.Errorf("the player's Tools menu is %v, want %v", player, want)
	}
}
func TestFogAndInitiativeAreTheGMsAlone(t *testing.T) {
	for _, m := range testRoomPage(room.RolePlayer).Menus() {
		if m.Label == "Fog" || m.Label == "Initiative" {
			t.Errorf("a player's bar carries %s; nothing under it is ever theirs", m.Label)
		}
	}
}
func TestEveryMenuCarriesItsItems(t *testing.T) {
	data := testRoomPage(room.RoleGM)
	for heading, want := range map[string][]string{
		"Tabletop":   {"Layers", "Grid & settings", "Spawn pawns", "Spawn from library", "Clear blood", "Clear drawing", "Clear tabletop"},
		"Fog":        {"Fill fog", "Clear fog"},
		"Initiative": {"Sync tracker", "Roll initiative", "Add entry", "Next turn", "Clear tracker"},
		"Tools":      {"Monster Manual", "Dice tray", "Music"},
		"View":       {"Zoom in", "Zoom out", "100%", "200%", "Fit map", "Toggle fullscreen"},
		"Help":       {"Settings", "Report issue", "Privacy policy", "Terms of service"},
	} {
		got := itemLabels(t, data, heading)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the %s menu is %v, want %v", heading, got, want)
		}
	}
}
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
		if item.Label == "Spawn pawns" {
			t.Error("a player's Tabletop menu offers Spawn pawns")
		}
	}
}
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
	items := menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items
	if items[len(items)-1].Label != "Clear tabletop" {
		t.Errorf("Clear tabletop is not the last item: %q", items[len(items)-1].Label)
	}
	for _, item := range menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items {
		if item.Label == "Clear tabletop" {
			t.Error("a player's Tabletop menu offers Clear tabletop")
		}
	}
}
func TestClearBloodIsEveryonesAndAsksTheRoomForNothing(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		var blood RoomMenuItem
		items := menuNamed(t, testRoomPage(role), "Tabletop").Items
		for _, item := range items {
			if item.Label == "Clear blood" {
				blood = item
			}
		}
		if blood.Label == "" {
			t.Fatalf("%s has no Clear blood: %v", role, itemLabels(t, testRoomPage(role), "Tabletop"))
		}
		if blood.Action != "clear-blood" {
			t.Errorf("%s's Clear blood has action %q, want clear-blood", role, blood.Action)
		}
		if blood.Post != "" || blood.Confirm != "" || blood.Danger {
			t.Errorf("%s's Clear blood is drawn as a mutation: %+v", role, blood)
		}
		if blood.Disabled {
			t.Errorf("%s's Clear blood is disabled", role)
		}
	}
	gm := itemLabels(t, testRoomPage(room.RoleGM), "Tabletop")
	if gm[len(gm)-1] != "Clear tabletop" {
		t.Errorf("the GM's Tabletop menu ends with %q, want Clear tabletop", gm[len(gm)-1])
	}
	page := markup(t, Room(testRoomPage(room.RolePlayer)))
	if !strings.Contains(page, `data-room-action="clear-blood"`) {
		t.Error("a player's page has no Clear blood the bar can dispatch")
	}
}
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
func TestAnOpenRoomRendersTheCanvasAndAClosedOneDoesNot(t *testing.T) {
	open := renderToString(t, roomContent(testRoomPage(room.RoleGM)))
	for _, want := range []string{"data-tabletop-canvas", "data-tabletop-unsupported", "touch-none"} {
		if !strings.Contains(open, want) {
			t.Errorf("the table has no %s:\n%s", want, open)
		}
	}
	if !strings.Contains(open, "data-tabletop-unsupported hidden") {
		t.Error("the unsupported message is not hidden to begin with")
	}
	closed := testRoomPage(room.RoleGM)
	closed.Closed = true
	if page := renderToString(t, roomContent(closed)); strings.Contains(page, "data-tabletop-canvas") {
		t.Errorf("a closed room drew a table:\n%s", page)
	}
}
func TestTheTableSettingsCrossAsAttributesThatAreThereOrAreNot(t *testing.T) {
	tests := []struct {
		name string
		attr string
		on   func(*RoomPageData)
	}{
		{name: "the camera", attr: "data-follow-turn", on: func(d *RoomPageData) { d.FollowTurn = true }},
		{name: "the blood", attr: "data-show-blood", on: func(d *RoomPageData) { d.ShowBlood = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := testRoomPage(room.RoleGM)
			tc.on(&data)
			if !strings.Contains(renderToString(t, roomContent(data)), tc.attr) {
				t.Errorf("the room page did not carry %s", tc.attr)
			}
		})
	}
	off := renderToString(t, roomContent(testRoomPage(room.RoleGM)))
	for _, tc := range tests {
		if strings.Contains(off, tc.attr) {
			t.Errorf("an account that turned %s off still got the attribute:\n%s", tc.attr, off)
		}
	}
}
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
func TestTheRoomMenuGivesTheGMTheRoomAndThePlayerTheDoor(t *testing.T) {
	gm := itemLabels(t, testRoomPage(room.RoleGM), "Room")
	want := []string{"Lock room", "Player List", "Copy room code", "Back to rooms", "Close room"}
	if strings.Join(gm, ",") != strings.Join(want, ",") {
		t.Errorf("the GM's Room menu is %v, want %v", gm, want)
	}
	player := itemLabels(t, testRoomPage(room.RolePlayer), "Room")
	want = []string{"Player List", "Copy room code", "Leave room"}
	if strings.Join(player, ",") != strings.Join(want, ",") {
		t.Errorf("the player's Room menu is %v, want %v", player, want)
	}
}
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
	if !strings.Contains(markup(t, Room(testRoomPage(room.RoleGM))), open) {
		t.Error("the page renders its own copy of the lock item")
	}
}
func TestThePlayersPageActsOnTheRoomInNoWayButLeaving(t *testing.T) {
	player := markup(t, Room(testRoomPage(room.RolePlayer)))
	for _, forbidden := range []string{"/lock", "/close"} {
		if strings.Contains(player, forbidden) {
			t.Errorf("the player's page carries the GM's %q", forbidden)
		}
	}
	if !strings.Contains(player, "/leave") {
		t.Error("the player's page has no way out of the room")
	}
	for _, want := range []string{"AB2C", "copy-code"} {
		if !strings.Contains(player, want) {
			t.Errorf("the player's page cannot copy the room code; %q is missing", want)
		}
	}
}
func TestTheHelpMenuOpensTheDocumentsInASecondTab(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	for _, path := range []string{"/privacy", "/tos"} {
		if !strings.Contains(page, `href="`+path+`" target="_blank" rel="noopener noreferrer"`) {
			t.Errorf("%s does not open in a second tab", path)
		}
	}
}
func TestSettingsOpensTheAccountDialogOverTheTable(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		if !strings.Contains(page, `data-modal-open="`+AccountSettingsPath+`"><span>Settings</span>`) {
			t.Errorf("%s cannot open the settings dialog from the Help menu", role)
		}
		if strings.Contains(page, `href="`+AccountSettingsPath) {
			t.Errorf("%s is sent to the settings dialog as a link; it is a dialog over the table", role)
		}
	}
}
func TestUnbuiltItemsAreDisabledRatherThanInert(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		page := markup(t, Room(testRoomPage(role)))
		for _, m := range testRoomPage(role).Menus() {
			for _, item := range m.Items {
				if !item.Disabled {
					continue
				}
				if !strings.Contains(page, "<button type=\"button\" disabled><span>"+item.Label+"</span>") {
					t.Errorf("%s's %q in the %s menu is not drawn as a disabled control", role, item.Label, m.Label)
				}
			}
		}
		if !strings.Contains(page, `class="menu-disabled"`) {
			t.Errorf("no disabled item on %s's bar carries menu-disabled", role)
		}
	}
}
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
	for _, forbidden := range []string{`id="table"`, `id="list"`, `id="status"`, `id="chat"`, `id="stack"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page carries %s, which is a DaisyUI component name", forbidden)
		}
	}
}

var toolButtons = regexp.MustCompile(`data-room-tool="([a-z]+)"[^>]*aria-pressed="true"`)

func TestTheToolPillStartsOnExactlyOneTool(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	if got := strings.Count(page, "data-room-tool="); got != len(RoomTools()) {
		t.Errorf("the pill has %d buttons, want %d", got, len(RoomTools()))
	}
	if got := len(toolButtons.FindAllString(page, -1)); got != 1 {
		t.Errorf("%d tools are pressed, want exactly 1", got)
	}
	pressed := toolButtons.FindStringSubmatch(page)
	if pressed == nil || pressed[1] != DefaultRoomTool {
		t.Errorf("the room does not open on %q: %v", DefaultRoomTool, pressed)
	}
	if strings.Index(page, `id="tabletop"`) > strings.Index(page, "data-room-tools") {
		t.Error("the tool pill is outside the table region")
	}
}
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
		if tool.Pans && tool.Measures {
			t.Errorf("%q both pans and measures", tool.Name)
		}
	}
	if measures != 1 {
		t.Errorf("%d tools in the list measure, want exactly 1", measures)
	}
}
func TestEveryToolShortcutIsItsOwnLetter(t *testing.T) {
	page := markup(t, Room(testRoomPage(room.RoleGM)))
	seen := map[string]string{}
	keyed := 0
	for _, tool := range RoomTools() {
		if tool.Key == "" {
			continue
		}
		keyed++
		if tool.Key != strings.ToLower(tool.Key) || len([]rune(tool.Key)) != 1 {
			t.Errorf("%q has shortcut %q, want one lower-case letter", tool.Name, tool.Key)
		}
		if other, taken := seen[tool.Key]; taken {
			t.Errorf("%q and %q both answer to %q", other, tool.Name, tool.Key)
		}
		seen[tool.Key] = tool.Name
		if !strings.Contains(page, `data-room-tool-key="`+tool.Key+`"`) {
			t.Errorf("%q renders no shortcut attribute:\n%s", tool.Name, page)
		}
		if !strings.Contains(page, ">"+tool.KeyLabel()+"</kbd>") {
			t.Errorf("%q does not print %q in its tooltip", tool.Name, tool.KeyLabel())
		}
	}
	if got := strings.Count(page, "data-room-tool-key="); got != keyed {
		t.Errorf("%d buttons carry a shortcut, want %d", got, keyed)
	}
	if seen[DefaultRoomTool] == "" && seen["v"] != DefaultRoomTool {
		t.Errorf("the default tool has no shortcut")
	}
}
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
	if strings.Index(gm, "data-layer-menu") < strings.Index(gm, "data-room-tools") {
		t.Error("the floors menu is rendered before the pill it belongs to")
	}
	if strings.Index(gm, `id="tabletop"`) > strings.Index(gm, "data-layer-menu") {
		t.Error("the floors menu is outside the table region")
	}
}
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
	for _, card := range []string{open, closed} {
		if !strings.Contains(card, "hx-confirm=") {
			t.Errorf("a room can be deleted without confirming it:\n%s", card)
		}
	}
}
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
		if !strings.HasPrefix(item.Window.URL, "/fragment/") {
			t.Errorf("%s: the window loads %q, which is not a fragment", role, item.Window.URL)
		}
	}
}
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

const testRoomIDText = "01BX5ZZKBKACTAV9WEVGEMMVT0"

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
	if !strings.Contains(page, `hx-confirm="`+data.KickPrompt(player)+`"`) {
		t.Errorf("the remove button has no confirm:\n%s", page)
	}
	if !strings.Contains(page, "Ilyana") {
		t.Error("the confirm does not name the person it is about")
	}
	if !strings.Contains(page, `hx-target="#room-members"`) {
		t.Errorf("the remove button does not target the list it replaces:\n%s", page)
	}
}
func TestAPlayerSeesNoRemoveButton(t *testing.T) {
	page := renderToString(t, RoomMembers(membersFor(false)))
	if strings.Contains(page, "hx-post=") {
		t.Errorf("the player list offers a player a remove button:\n%s", page)
	}
}

func debugRoomPage(role room.Role) RoomPageData {
	data := testRoomPage(role)
	data.Debug = true
	return data
}
func TestTheDebugMenuIsOnlyThereOnADevelopmentBuild(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		for _, label := range menuLabels(testRoomPage(role)) {
			if label == "Debug" {
				t.Errorf("a %s sees a Debug menu on a build that is not a development one", role)
			}
		}
		labels := menuLabels(debugRoomPage(role))
		if last := labels[len(labels)-1]; last != "Debug" {
			t.Errorf("a %s's development bar ends with %q, want Debug", role, last)
		}
	}
}
func TestTheDebugMenuOpensOneWindowPerSurface(t *testing.T) {
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		got := itemLabels(t, debugRoomPage(role), "Debug")
		want := []string{"Renderer", "Events", "State", "Server"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("a %s's Debug menu is %v, want %v", role, got, want)
		}
	}
}
func TestEveryDebugSurfaceIsAWindowOnAFragment(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range menuNamed(t, debugRoomPage(room.RoleGM), "Debug").Items {
		if item.Window.ID == "" {
			t.Errorf("%q is not a window, so it cannot sit beside the table it reports on", item.Label)
			continue
		}
		if seen[item.Window.ID] {
			t.Errorf("%q reuses the window id %q, so the two would share a position", item.Label, item.Window.ID)
		}
		seen[item.Window.ID] = true
		if !strings.HasPrefix(item.Window.URL, "/fragment/") {
			t.Errorf("the %s window loads %q, which the client refuses", item.Label, item.Window.URL)
		}
	}
}
func roomMenuItem(d RoomPageData, label string) (RoomMenuItem, bool) {
	for _, menu := range d.Menus() {
		for _, item := range menu.Items {
			if item.Label == label {
				return item, true
			}
		}
	}
	return RoomMenuItem{}, false
}
func TestOnlyAPlayerWithACharacterIsOfferedTheSheet(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	find := roomMenuItem
	player := RoomPageData{ID: id, Role: room.RolePlayer, CharacterID: id, CharacterName: "Ilyana"}
	item, ok := find(player, "Character sheet")
	if !ok {
		t.Fatal("a player with a character is offered no sheet")
	}
	if item.Disabled || item.Window.ID != SheetWindow || item.Window.Title != "Ilyana" {
		t.Errorf("the sheet item is %+v", item)
	}
	if !strings.HasPrefix(item.Window.URL, "/fragment/") {
		t.Errorf("the sheet window loads %q, which a window refuses", item.Window.URL)
	}
	seatless := RoomPageData{ID: id, Role: room.RolePlayer}
	if item, _ := find(seatless, "Character sheet"); !item.Disabled {
		t.Error("a player with no character is offered a live sheet")
	}
	gm := RoomPageData{ID: id, Role: room.RoleGM}
	if _, ok := find(gm, "Character sheet"); ok {
		t.Error("the GM is offered a character sheet")
	}
}

func TestOnlyAPlayerWithACharacterIsOfferedTheirJournal(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	player := RoomPageData{ID: id, Role: room.RolePlayer, CharacterID: id, CharacterName: "Ilyana"}
	item, ok := roomMenuItem(player, "Journal")
	if !ok {
		t.Fatal("a player with a character is offered no journal")
	}
	if item.Disabled || item.Window.ID != JournalWindow {
		t.Errorf("the journal item is %+v", item)
	}
	if item.Window.ID == SheetWindow {
		t.Error("the journal and the sheet share a window id, so one replaces the other")
	}
	if !strings.HasPrefix(item.Window.URL, "/fragment/") {
		t.Errorf("the journal window loads %q, which a window refuses", item.Window.URL)
	}
	seatless := RoomPageData{ID: id, Role: room.RolePlayer}
	if item, _ := roomMenuItem(seatless, "Journal"); !item.Disabled {
		t.Error("a player with no character is offered a journal to write in")
	}
	gm := RoomPageData{ID: id, Role: room.RoleGM}
	if _, ok := roomMenuItem(gm, "Journal"); ok {
		t.Error("the GM is offered a character journal")
	}
}

func TestOnlyTheGMIsOfferedTheMonsterManual(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	gm := RoomPageData{ID: id, Role: room.RoleGM}
	item, ok := roomMenuItem(gm, "Monster Manual")
	if !ok {
		t.Fatal("the GM is offered no manual")
	}
	if item.Disabled || item.Window.ID != ManualWindow {
		t.Errorf("the manual item is %+v", item)
	}
	if !strings.HasPrefix(item.Window.URL, "/fragment/") {
		t.Errorf("the manual window loads %q, which a window refuses", item.Window.URL)
	}
	if strings.Contains(item.Window.URL, id) {
		t.Errorf("the manual window is keyed to a room in %q, so a GM's layout would not follow them between tables", item.Window.URL)
	}
	player := RoomPageData{ID: id, Role: room.RolePlayer, CharacterID: id}
	if _, ok := roomMenuItem(player, "Monster Manual"); ok {
		t.Error("a player is offered the GM's manual")
	}
}
