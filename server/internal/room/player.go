package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE PLAYER FAMILY, and with it the room's own two events and the six
// hub-only commands.
//
// THE HUB-ONLY REGISTRY IS THE POINT OF THIS FILE. player.join carries a whole
// Player, role included, so a browser that could send it could make itself the
// GM. It cannot, and the reason is structural rather than a check somebody had
// to remember: DecodeCommand looks only in wireCommands, these are in
// hubCommands, and a frame naming one of them comes back as an unknown type.
// The hub builds them from the connection it authenticated and from the HTTP
// handlers that already own the rooms row.
//
// THE ROOM ROW IS THE WRITER OF RECORD for the lock, the name and the closing.
// The HTTP route writes the database and then tells the hub, which mirrors it
// into state and broadcasts. Going the other way would put two writers on one
// fact and make "is this room locked" a question with two answers.

// PlayerJoined announces somebody new.
type PlayerJoined struct {
	Header
	Player Player `json:"player"`
}

func (*PlayerJoined) eventType() string { return "player.joined" }

// PlayerUpdated is any change to a player, including the connected flag. There
// is no separate "player disconnected" event: the toast is derived from this
// one, so what the client says and what it knows cannot disagree.
type PlayerUpdated struct {
	Header
	Player Player `json:"player"`
}

func (*PlayerUpdated) eventType() string { return "player.updated" }

// PlayerLeft names somebody who is gone for good, as opposed to disconnected.
type PlayerLeft struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*PlayerLeft) eventType() string { return "player.left" }

// PlayerKicked goes to one person and tells them why their socket is about to
// close. It is transient: there is nothing about it to restore, because the
// client it reaches is leaving.
type PlayerKicked struct {
	Header
	Reason string `json:"reason"`
}

func (*PlayerKicked) eventType() string { return "player.kicked" }
func (*PlayerKicked) Transient() bool   { return true }

// RoomUpdated carries the room's name and lock state.
type RoomUpdated struct {
	Header
	Room RoomInfo `json:"room"`
}

func (*RoomUpdated) eventType() string { return "room.updated" }

// RoomClosed tells everybody the table is over. It is transient for the same
// reason as a kick: every client that receives it is on its way out.
type RoomClosed struct {
	Header
}

func (*RoomClosed) eventType() string { return "room.closed" }
func (*RoomClosed) Transient() bool   { return true }

// PlayerKick removes somebody from the table. It is the one command in this
// file a browser may send.
type PlayerKick struct {
	ID ulid.ULID `json:"id"`
}

// KickReason is what the removed player is told. It is deliberately flat: the
// GM is not asked to type a reason, because a text box in front of a moderation
// action is an invitation to write something they would rather not have sent.
const KickReason = "The GM removed you from the room."

func (c *PlayerKick) Authorize(s *State, a Actor) error {
	if err := requireGM(a, "remove somebody from the room"); err != nil {
		return err
	}

	// The GM cannot kick themselves, and not because of a role check on the
	// target's row -- because a room with nobody who can unlock it, close it or
	// reopen it is a room that has to be abandoned.
	if c.ID == a.ID {
		return forbidden("Not yourself", "The GM cannot leave their own room this way. Close it instead.")
	}

	return nil
}

func (c *PlayerKick) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p := s.Player(c.ID)
	if p == nil {
		return nil, notFound("Nobody there", "That player is no longer in the room.")
	}
	if p.Role == RoleGM {
		return nil, forbidden("Not the GM", "The GM cannot be removed from their own room.")
	}

	// THEIR PAWNS STAY. A character standing in the middle of a fight is part
	// of the board, and deleting it because the person controlling it left
	// would rearrange the encounter. The GM removes the pawn if they want it
	// gone, with the command for removing pawns.
	s.Players = slices.DeleteFunc(s.Players, func(q Player) bool { return q.ID == c.ID })
	s.Normalize()

	// The kicked player is told first, because the hub closes their
	// connections on seeing it and the broadcast that follows is not for them.
	return []Emission{
		toPlayer(c.ID, &PlayerKicked{Reason: KickReason}),
		to(ToAll, &PlayerLeft{ID: c.ID}),
	}, nil
}

// PlayerJoin seats somebody, or seats them again.
//
// HUB ONLY. The Player it carries was built from an authenticated session, and
// its Role came from comparing the user against the rooms row's owner.
type PlayerJoin struct {
	Player Player `json:"player"`
}

// Authorize is nil for every hub-only command. There is no actor to check:
// these are not sent by anybody, they are what the server does when a socket
// opens or an HTTP handler finishes.
func (c *PlayerJoin) Authorize(s *State, a Actor) error { return nil }

func (c *PlayerJoin) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p := clonePlayer(c.Player)
	p.Connected = true

	if existing := s.Player(p.ID); existing != nil {
		*existing = p
		s.Normalize()

		// A RETURNING PLAYER IS AN UPDATE, NOT A JOIN. Their row never left,
		// so their pawns and their turn are still where they were, and a
		// joined event would have every client treat a reconnect as a new
		// arrival -- a toast, a row appended, a sound.
		return []Emission{to(ToAll, &PlayerUpdated{Player: clonePlayer(p)})}, nil
	}

	s.Players = append(s.Players, p)
	s.Normalize()

	return []Emission{to(ToAll, &PlayerJoined{Player: clonePlayer(p)})}, nil
}

// PlayerSetConnected flips the flag a lost connection sets. HUB ONLY.
type PlayerSetConnected struct {
	ID        ulid.ULID `json:"id"`
	Connected bool      `json:"connected"`
}

func (c *PlayerSetConnected) Authorize(s *State, a Actor) error { return nil }

func (c *PlayerSetConnected) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p := s.Player(c.ID)
	if p == nil {
		return nil, nil
	}

	// THE ROW STAYS. Losing wifi in the middle of a fight must not delete the
	// player: their pawns are owned by this id and their line in the tracker
	// names it, so a disconnect is one field changing and a reconnect is the
	// same field changing back.
	p.Connected = c.Connected
	s.Normalize()

	return []Emission{to(ToAll, &PlayerUpdated{Player: clonePlayer(*p)})}, nil
}

// PlayerLeave removes somebody who left on purpose, through the room page's own
// Leave button rather than through a closed socket. HUB ONLY.
type PlayerLeave struct {
	ID ulid.ULID `json:"id"`
}

func (c *PlayerLeave) Authorize(s *State, a Actor) error { return nil }

func (c *PlayerLeave) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if s.Player(c.ID) == nil {
		return nil, nil
	}

	s.Players = slices.DeleteFunc(s.Players, func(q Player) bool { return q.ID == c.ID })
	s.Normalize()

	return []Emission{to(ToAll, &PlayerLeft{ID: c.ID})}, nil
}

// RoomSetLocked mirrors the rooms row's locked column. HUB ONLY.
type RoomSetLocked struct {
	Locked bool `json:"locked"`
}

func (c *RoomSetLocked) Authorize(s *State, a Actor) error { return nil }

func (c *RoomSetLocked) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	s.Room.Locked = c.Locked
	s.Normalize()

	return []Emission{to(ToAll, &RoomUpdated{Room: s.Room})}, nil
}

// RoomSetName mirrors the rooms row's name column. HUB ONLY.
type RoomSetName struct {
	Name string `json:"name"`
}

func (c *RoomSetName) Authorize(s *State, a Actor) error { return nil }

func (c *RoomSetName) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if err := checkRequiredName("room", c.Name); err != nil {
		return nil, err
	}

	s.Room.Name = c.Name
	s.Normalize()

	return []Emission{to(ToAll, &RoomUpdated{Room: s.Room})}, nil
}

// RoomClose ends the session. The hub drops every connection and unloads the
// room after this goes out. HUB ONLY.
type RoomClose struct{}

func (c *RoomClose) Authorize(s *State, a Actor) error { return nil }

func (c *RoomClose) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	return []Emission{to(ToAll, &RoomClosed{})}, nil
}

func clonePlayer(p Player) Player {
	p.CharacterID = cloneID(p.CharacterID)

	return p
}
