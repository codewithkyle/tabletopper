package pages

import (
	"strconv"

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
}

// IsGM is the one question the markup asks of the role, written here so that
// the comparison lives beside the type rather than in a template.
func (d RoomPageData) IsGM() bool {
	return d.Role == room.RoleGM
}

// RoomMenu is one heading in the bar and what drops out of it.
type RoomMenu struct {
	Label string
	Items []RoomMenuItem
}

// RoomMenuItem is one line in a menu. It is a struct of alternatives rather
// than an interface because there are only four ways an item can behave and the
// markup has to switch on them anyway:
//
//   - Disabled: a feature that does not exist yet. Nothing else is read.
//   - Href: ordinary navigation. NewTab sends it to a second tab.
//   - Post: a mutation, over htmx, with the confirm modal in front of the
//     destructive ones.
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

	items = append(items, RoomMenuItem{Label: "Player List", Disabled: true})

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
