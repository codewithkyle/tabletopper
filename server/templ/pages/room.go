package pages

import "tabletopper/internal/room"

// THE ROOM SHELL, which is the page every later phase of the virtual tabletop
// mounts on. Nothing here is live: the members panel is what the database says
// at the moment the page was rendered, and the two panels below it are labelled
// placeholders. The socket, the canvas and the hub arrive in later phases and
// arrive INSIDE this layout rather than replacing it.
//
// THE PAGE IS A GRID OF TWO ROWS AND THE BODY IS TWO COLUMNS. The header row
// carries what the room is and what may be done to it; the body is the table
// region on the left and a side panel on the right. That split is the one the
// whole VTT is built around: the canvas owns map space -- tiles, grid, fog,
// pawns, anything that moves when the camera moves -- and the DOM owns screen
// space, which is every panel on the right.
//
// THE TABLE REGION IS id="tabletop" AND NOT id="table". `table` is a DaisyUI
// component, and any bare occurrence of it in a .templ file -- inside an
// attribute value included -- emits the whole table family into the built
// stylesheet with nothing failing. The same goes for `list`, `status`, `tab`,
// `stack`, `swap`, `menu` and `link`, which is why the ids below read tabletop,
// members and room-code.
//
// ALL OF THE REASONING ABOUT THIS PAGE LIVES IN THIS FILE, for that same
// reason: room.templ cannot carry a comment of any kind.

// roomLockID is the element the two lock routes swap. It is a constant because
// three attributes name it -- the control's own id and the hx-target on each of
// the two buttons inside it -- and a target that has drifted from its id fails
// silently: htmx finds nothing to swap and the button stops working.
const roomLockID = "room-lock"

// RoomPageData is the whole page, with every conversion already done. The
// controller turns a nullable code column, a nullable closed_at and an owner id
// into a string, two bools and a role, so the markup asks nothing of the
// database's types and the empty struct renders an empty page.
type RoomPageData struct {
	ID   string
	Name string

	// Code is empty for a closed room, and only the GM is ever shown it. A
	// player who is already in the room has no use for it, and a room's code is
	// the one thing on this page that admits somebody else.
	Code string

	Locked bool
	Closed bool

	// Role is what this viewer may do, derived from rooms.owner_id rather than
	// stored anywhere. The GM gets the code, the lock and Close; a player gets
	// Leave. It is room.Role rather than a bool because the protocol phase
	// authorises every command against the same type.
	Role room.Role

	Members RoomMembersData
}

// IsGM is the one question the markup asks of the role, written here so that
// the comparison lives beside the type rather than in a template.
func (d RoomPageData) IsGM() bool {
	return d.Role == room.RoleGM
}

// back is where the bar's link goes, and it differs by role because the page
// above this one differs by role. The GM came from their rooms; a player came
// from the join page, and sending them to /rooms would show them a list of
// their own rooms, which is not where they were and is probably empty.
func (d RoomPageData) back() backTarget {
	if d.IsGM() {
		return roomsBack()
	}

	return homeBack()
}

// RoomMembersData is the members panel, which is its own type because it is
// also a fragment: the panel refetches itself, so the page and that route
// render the same component from the same struct.
//
// IT CARRIES THE ROOM ID because the panel's own refetch URL is built from it,
// and a fragment that did not know which room it was showing would have to be
// told by whatever swapped it in.
type RoomMembersData struct {
	RoomID  string
	Members []RoomMember
}

// RoomMember is one person at the table as the panel draws them: who they are,
// and what they brought.
//
// CHARACTER IS A NAME AND NOT AN ID, because the panel is read by people. It is
// empty for the two cases that are not errors -- somebody who joined with no
// character, and somebody whose character was deleted after they joined -- and
// the panel says so rather than printing a blank.
type RoomMember struct {
	Name      string
	AvatarURL string
	Character string
}
