package room

// Role is what one person may do at one table. There are two, and the room row
// is what decides which: the owner is the GM and everybody else who got in is a
// player. It is not stored anywhere -- storing it would be storing a second
// copy of rooms.owner_id, which is the pairing 20260906190000 refused for
// shares and which is refused again here.
//
// IT IS A NAMED TYPE RATHER THAN A BOOL because the protocol phase authorises
// every command against it, and `if role == RoleGM` reads as what it is where
// `if isGM` reads as a fact about a person. There is no third member on the
// roadmap: co-GMs and spectators were both considered and both deferred, and
// each would arrive as a member here rather than as a second flag beside it.
type Role string

const (
	// RoleGM is the room's owner. The GM sees hidden pawns and every layer,
	// and is the only role that can change the table.
	RoleGM Role = "gm"

	// RolePlayer is anybody whose session is pointed at this room. A player
	// sees the active layer and what is visible on it, and moves what they own.
	RolePlayer Role = "player"
)
