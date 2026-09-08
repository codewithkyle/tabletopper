package pages

import (
	"slices"
	"strconv"
	"strings"

	"tabletopper/internal/room"
)

// THE ROOM IS ONE APPLICATION WINDOW, AND ITS CHROME IS A MENU BAR. The table
// fills the viewport and everything the GM can do to it hangs off a thin bar
// across the top, the way a desktop application's does -- because that is what
// this page is. A VTT is not a document with controls beside it; it is a
// workspace, and a workspace that spends a fifth of its width on panels is a
// workspace with a fifth less table.
//
// SO THERE IS NO SIDE PANEL. The members list, the initiative tracker and the
// chat placeholder that used to sit down the right are gone: the player list
// becomes a window opened from the Room menu, initiative is not a list beside
// the map, and there is no chat. Each of those is a menu item here and a
// feature later, which is the whole point of writing the bar first -- the shape
// of the application is settled before any of it is built.
//
// MOST ITEMS ARE DISABLED AND THAT IS THE HONEST STATE. There is no canvas yet,
// so there is nothing to zoom, no fog to fill and no pawn to spawn. A disabled
// item says "this belongs here and does not work yet"; an enabled one that does
// nothing says "this is broken". A desktop menu greys items out constantly and
// nobody reads that as a defect.
//
// ALL OF THE REASONING ABOUT THIS PAGE LIVES IN THIS FILE. room.templ cannot
// carry a comment of any kind: Tailwind reads every .templ file as text and
// takes a class-name candidate from every word in it, so an ordinary English
// sentence about "the table" or "the player list" emits a DaisyUI component
// family into the built stylesheet and nothing anywhere fails.
//
// THE TABLE REGION IS id="tabletop" AND NOT id="table", for exactly that
// reason: `table` is a DaisyUI component and an attribute value is scanned like
// anything else. The same goes for `list`, `status`, `tab`, `stack`, `swap`,
// `menu` and `chat` as bare lower-case words.

// roomLockID is the menu item the two lock routes swap. It is a constant
// because the item carries it as an id and derives its own hx-target from it,
// and a target that has drifted from its id fails silently: htmx finds nothing
// to swap and the item stops changing.
const roomLockID = "room-lock"

// DefaultRoomTool is the pointer mode a room opens in. Move, because the first
// thing anybody does at a table is push something around, and because it is the
// only one of the four that cannot damage anything.
const DefaultRoomTool = "move"

// RoomPageData is the whole page, with every conversion already done. The
// controller turns a nullable code column, a nullable closed_at and an owner id
// into a string, two bools and a role, so the markup asks nothing of the
// database's types and the empty struct renders an empty page.
type RoomPageData struct {
	ID   string
	Name string

	// Code is empty for a closed room, and only the GM is ever given it. A
	// player who is already in the room has no use for it, and it is the one
	// thing on this page that admits somebody else.
	Code string

	Locked bool
	Closed bool

	// Role is what this viewer may do, derived from rooms.owner_id rather than
	// stored anywhere. It is room.Role rather than a bool because the protocol
	// phase authorises every command against the same type.
	Role room.Role

	// Socket is the path the client connects to, and it is empty for a closed
	// room. That emptiness is the whole of "do not connect": the client reads
	// the attribute and does nothing when it is not there, which is one
	// condition in one place rather than a reconnect loop against a room the
	// hub will refuse to load.
	Socket string

	// Version is the server build. It goes on the bundle URL, so a deploy
	// changes the URL and the one-hour cache on /static/ cannot answer the
	// reload with the script that was there before it.
	Version string

	// Debug renders the development panel: connection state, sequence number,
	// the player list and the last twenty frames, with a box to send a raw
	// command. It is the config's Development and nothing else, so it cannot
	// be turned on from a query string.
	Debug bool
}

// Bundle is the room module's URL with the build on it. It is a method rather
// than a field because the two halves must not be able to drift: whatever
// version the page reports in its snapshot comparison is the version whose
// bundle it loaded.
func (d RoomPageData) Bundle() string {
	if d.Version == "" {
		return "/static/room.js"
	}

	return "/static/room.js?v=" + d.Version
}

// MembersPath is the fragment the player window fetches itself from. The room
// travels as a query parameter rather than in the path because this is a
// representation of a room's membership and not a resource of its own -- the
// same reason the two share dialogs read one.
func (d RoomPageData) MembersPath() string {
	return "/fragment/room/members?room=" + d.ID
}

// IsGM is the one question the markup asks of the role, written here so that
// the comparison lives beside the type rather than in a template.
func (d RoomPageData) IsGM() bool {
	return d.Role == room.RoleGM
}

// RoleName is the role as the client reads it off the mount element. It is a
// conversion and not a cast in the markup, because room.Role is a string type
// and templ takes a string -- and because the two values it can hold are the
// same two the protocol validates, which is the point of it not being a bool.
func (d RoomPageData) RoleName() string {
	return string(d.Role)
}

// RoomMenu is one heading in the bar and what drops out of it.
type RoomMenu struct {
	Label string
	Items []RoomMenuItem
}

// RoomMenuItem is one line in a menu. It is a struct of alternatives rather
// than an interface because there are only five ways an item can behave and the
// markup has to switch on them anyway:
//
//   - Disabled: a feature that does not exist yet. Nothing else is read.
//   - Href: ordinary navigation. NewTab sends it to a second tab.
//   - Post: a mutation, over htmx, with the confirm modal in front of the
//     destructive ones.
//   - Window: opens a floating panel over the table on a fragment URL.
//   - Action: a behaviour that is entirely client-side, named for room.js.
//
// ID IS BOTH AN ANCHOR AND A CONTRACT. An item that carries one also carries an
// hx-target pointing at itself, so a mutation answers with the item it just
// changed -- which is how Lock becomes Unlock without redrawing the page. Only
// the lock item uses it.
type RoomMenuItem struct {
	Label string
	ID    string

	Href   string
	NewTab bool

	Post           string
	Confirm        string
	ConfirmHeading string
	ConfirmLabel   string

	Action string
	Value  string

	// Window is the floating panel this item opens, and an item that carries
	// one carries nothing else. See RoomWindow.
	Window RoomWindow

	// Danger marks the one destructive item in a menu, which is drawn in the
	// error colour and sits last.
	Danger bool

	Disabled bool
}

// Menus is the whole bar, in order. The seven headings are fixed; what varies
// inside them is the Room menu, which is the only one whose contents depend on
// who is looking and on whether the room is open.
func (d RoomPageData) Menus() []RoomMenu {
	return []RoomMenu{
		d.roomMenu(),
		{Label: "Tabletop", Items: comingSoon("Settings", "Load image", "Spawn pawns", "Clear tabletop")},
		{Label: "Fog", Items: comingSoon("Fill fog", "Clear fog")},
		{Label: "Initiative", Items: comingSoon("Sync tracker", "Clear tracker")},
		{Label: "Window", Items: comingSoon("Monster Manual", "Dice tray")},
		d.viewMenu(),
		helpMenu(),
	}
}

// roomMenu is the room itself, and it is three different menus.
//
// THE GM OWNS THE ROOM AND A PLAYER IS ONLY IN IT. So the GM gets the lock, the
// code and the close, and a player gets a way out -- there is nothing else a
// player may do to a room they do not own. A closed room is a fourth case
// inside the first: its code is gone, so there is nothing to copy, nothing to
// lock and nothing left to close.
//
// BACK TO ROOMS IS HERE RATHER THAN AS AN ARROW IN THE CORNER, which is the one
// place this page departs from every other page in the app. A menu bar owns the
// top-left, and a GM stepping away from a table is not the same act as closing
// it -- so the two sit next to each other and only one of them is destructive.
func (d RoomPageData) roomMenu() RoomMenu {
	items := []RoomMenuItem{}

	if d.IsGM() && !d.Closed {
		items = append(items, roomLockItem(d))
	}
	if d.IsGM() && d.Closed {
		items = append(items, RoomMenuItem{Label: "Reopen room", Post: "/rooms/" + d.ID + "/open"})
	}

	items = append(items, RoomMenuItem{Label: "Player List", Window: RoomWindow{
		ID:     "players",
		Title:  "Players",
		URL:    d.MembersPath(),
		Width:  260,
		Height: 260,
	}})

	if d.IsGM() {
		if !d.Closed {
			items = append(items, RoomMenuItem{Label: "Copy room code", Action: "copy-code", Value: d.Code})
		}
		items = append(items, RoomMenuItem{Label: "Back to rooms", Href: "/rooms"})
		if !d.Closed {
			items = append(items, RoomMenuItem{
				Label:          "Close room",
				Post:           "/rooms/" + d.ID + "/close",
				Confirm:        "Everyone in this room will be removed and the code will stop working.",
				ConfirmHeading: "Close this room?",
				ConfirmLabel:   "Close room",
				Danger:         true,
			})
		}

		return RoomMenu{Label: "Room", Items: items}
	}

	return RoomMenu{Label: "Room", Items: append(items, RoomMenuItem{
		Label:  "Leave room",
		Post:   "/rooms/" + d.ID + "/leave",
		Danger: true,
	})}
}

// roomLockItem is the one item that answers a mutation with itself. Locking a
// room and unlocking it are two routes and one line in the menu, so the reply
// swaps the line rather than the page -- and because it is built here, the
// route's reply and the page's first render are the same markup.
func roomLockItem(d RoomPageData) RoomMenuItem {
	if d.Locked {
		return RoomMenuItem{ID: roomLockID, Label: "Unlock room", Post: "/rooms/" + d.ID + "/unlock"}
	}

	return RoomMenuItem{ID: roomLockID, Label: "Lock room", Post: "/rooms/" + d.ID + "/lock"}
}

// viewMenu is the camera, plus the one item in it that needs no camera.
// Fullscreen is the browser's own and works today; everything else moves a
// viewport that does not exist yet.
func (d RoomPageData) viewMenu() RoomMenu {
	return RoomMenu{Label: "View", Items: []RoomMenuItem{
		{Label: "Zoom in", Disabled: true},
		{Label: "Zoom out", Disabled: true},
		{Label: "100%", Disabled: true},
		{Label: "200%", Disabled: true},
		{Label: "Toggle fullscreen", Action: "fullscreen"},
		{Label: "Center tabletop", Disabled: true},
	}}
}

// helpMenu is the two documents every page in the app already links to, and the
// issue report that does not exist yet.
//
// BOTH OPEN IN A SECOND TAB, which is the one place in this app that is true.
// Everywhere else these are ordinary links; here, following one would take
// somebody out of a game that is in progress, and coming back is a navigation
// rather than a close.
func helpMenu() RoomMenu {
	return RoomMenu{Label: "Help", Items: []RoomMenuItem{
		{Label: "Report issue", Disabled: true},
		{Label: "Privacy policy", Href: "/privacy", NewTab: true},
		{Label: "Terms of service", Href: "/tos", NewTab: true},
	}}
}

// comingSoon is every item whose feature is not built. They are written as a
// list of labels because that is all there is to them, and they are in the bar
// at all because the shape of the application is a decision worth making before
// the features are -- a menu that grows an item later moves everything under
// it, and a GM learns where things are by muscle memory.
func comingSoon(labels ...string) []RoomMenuItem {
	items := make([]RoomMenuItem, 0, len(labels))
	for _, label := range labels {
		items = append(items, RoomMenuItem{Label: label, Disabled: true})
	}

	return items
}

// RoomTool is one pointer mode in the floating toolbar: what a click and a drag
// on the table do.
//
// IT IS A TOOLBAR AND NOT A MENU because a pointer mode is switched constantly
// while both hands are busy, and a mode you change twenty times a minute cannot
// live two clicks deep. It floats over the table rather than sitting in the bar
// for the same reason: it belongs to the surface it acts on.
//
// NOTHING READS THE SELECTION YET. The toolbar keeps its own state -- which
// button is pressed -- and the canvas that will ask it does not exist. That
// makes it a real control over a feature that is not built, rather than a
// picture of one.
type RoomTool struct {
	Name  string
	Label string
}

// RoomTools is the four modes, in the order a hand reaches for them.
func RoomTools() []RoomTool {
	return []RoomTool{
		{Name: DefaultRoomTool, Label: "Move"},
		{Name: "measure", Label: "Measure"},
		{Name: "fog", Label: "Fog"},
		{Name: "draw", Label: "Draw"},
	}
}

// Pressed reports whether this tool is the one a freshly loaded room starts on.
// It is a method rather than an index comparison in the markup so that the
// default is named in one place.
func (t RoomTool) Pressed() string {
	return strconv.FormatBool(t.Name == DefaultRoomTool)
}

// RoomWindow is a floating panel over the table: the player list, a monster's
// stat block, the layer manager. It is what a menu item carries instead of a
// route.
//
// A WINDOW IS NOT A MODAL AND MUST NOT BECOME ONE. The three <dialog> modals
// are one at a time, block the page, and are dismissed; a window blocks
// nothing, sits where the GM put it, and several are open at once while they
// work. See the Windows section in CLAUDE.md for why this one is allowed the
// corner controls that a modal is not.
//
// IT IS THREE STRINGS AND NO MARKUP. The client clones the chrome from a
// <template> and loads URL into it with htmx, so anything already served under
// /fragment/ can be a window without a line of server change -- and a fragment
// that refetches itself on a socket event goes on doing that inside one.
type RoomWindow struct {
	// ID is the stable identity: one window per id, and the key its position
	// and size are remembered under. It is deliberately not the URL, which
	// carries the room and would key a GM's layout per table.
	ID    string
	Title string
	URL   string

	// Width and Height are the size a window opens at the FIRST time somebody
	// opens it. After that the size they left it at wins.
	Width  int
	Height int
}

// WidthValue and HeightValue render the two optional attributes, empty when
// unset so the markup can leave them off entirely. They are methods rather than
// a strconv call in the template because a .templ file is scanned by Tailwind
// as text and every import is one more file to keep prose out of.
func (w RoomWindow) WidthValue() string { return dimension(w.Width) }

func (w RoomWindow) HeightValue() string { return dimension(w.Height) }

func dimension(value int) string {
	if value <= 0 {
		return ""
	}

	return strconv.Itoa(value)
}

// RoomMember is one person at the table as the player window draws them. It is
// not room.Player: that type carries ids and a character reference this window
// has no use for, and a template that took it would be able to render either.
//
// IT IS TWO NAMES, AND THE WINDOW DRAWS BOTH -- "Ilyana Vasilovich (kyle)" for
// a player and "Game Master (kyle)" for the person running it.
//
// THE CHARACTER LEADS AND THE ACCOUNT FOLLOWS, because for the next four hours
// the character is what everybody at the table is going to say out loud, and
// the account is how the GM tells two of them apart when both players are
// called Bob -- or works out whose socket to close. A list of usernames would
// have the useful half in brackets.
//
// "GAME MASTER" IS NOT A CHARACTER AND IS NOT PRETENDING TO BE ONE. It goes in
// the same slot because the GM occupies the same kind of seat, and because a
// row with an empty first half and a name in brackets reads as a bug.
type RoomMember struct {
	// Name is the line's first half: the character, or "Game Master", or the
	// account name again when there is no character to show.
	Name string

	// Username is the account, drawn in brackets after the name. It is never
	// empty -- see clerkauth.FallbackUsername for the reason that holds.
	Username string

	Avatar string

	// IsGM sorts them to the top and names them, because "who is running
	// this" is the first thing anybody wants from a player list.
	IsGM bool

	// Connected is false for somebody whose socket has dropped and whose row
	// is being kept for them. It is always false in the fallback list, which is
	// honest: a room that is not running has nobody connected to it.
	Connected bool
}

// THE REFETCH PATTERN HAS ONE RACE AND hx-sync IS THE ANSWER TO IT. A burst of
// player events fires a burst of GETs, and two responses can land in either
// order -- the socket is ordered, a pair of HTTP requests is not -- which would
// leave the window showing whichever answer arrived last rather than the newest
// one.
//
// "queue last" AND NOT "replace", which was the first thing written here and
// was wrong twice over. replace aborts the request in flight, and htmx reports
// every cancellation as an error -- so the ordinary page load, where the load
// trigger's fetch is still open when the first snapshot fires room:players,
// wrote a stack trace to the console. Worse, under a sustained burst each new
// event would restart a request that then never finished. queue last runs one
// at a time and keeps only the newest pending one, which is the same guarantee
// without cancelling anything.
//
// RoomMembersData is the window's whole contents.
type RoomMembersData struct {
	RoomID  string
	Members []RoomMember

	// Live says the list came from the running room rather than from the
	// session rows. The difference is visible -- the fallback cannot see the
	// GM at all, whose membership is ownership of the rooms row rather than a
	// room_id on their session -- so the window says which it is showing
	// instead of quietly presenting one as the other.
	Live bool
}

// Path is the fragment's own URL, so the swapped-in copy refetches itself the
// same way the first one did.
func (d RoomMembersData) Path() string {
	return "/fragment/room/members?room=" + d.RoomID
}

// GameMasterName is the first half of the GM's line. It is a constant here
// rather than a string in the markup because both halves of the member list --
// the live one out of the hub and the fallback out of the session rows -- build
// it, and two spellings of it would be two different rooms.
const GameMasterName = "Game Master"

// MemberName is the line a member is drawn under: the GM's title, the character
// they brought, or their account name when there is no character.
//
// THE LAST CASE IS NOT A PLACEHOLDER. A player whose character was deleted
// while they were away is still at the table, and "(kyle)" with nothing before
// it is worse than their name twice -- so the caller writes the account name
// into both halves and the row reads as somebody with no character rather than
// as a row that failed to load.
func MemberName(isGM bool, character string, username string) string {
	if isGM {
		return GameMasterName
	}
	if character != "" {
		return character
	}

	return username
}

// ShowUsername is false when the account is already the whole line, which is
// what MemberName falls back to for somebody with no character. "rin (rin)" is
// not more informative than "rin", it is just noisier, and a reader scanning a
// list of them would spend a moment on every one working out that the two
// halves are the same word.
func (m RoomMember) ShowUsername() bool {
	return m.Name != m.Username
}

// SortRoomMembers puts the GM first and everybody else in name order, which is
// stable across refetches -- a list that reordered itself every time somebody
// reconnected would be a list nobody could read.
func SortRoomMembers(members []RoomMember) []RoomMember {
	slices.SortFunc(members, func(a, b RoomMember) int {
		if a.IsGM != b.IsGM {
			if a.IsGM {
				return -1
			}

			return 1
		}

		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return members
}
