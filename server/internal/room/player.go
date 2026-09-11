package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

type PlayerJoined struct {
	Header
	Player Player `json:"player"`
}

func (*PlayerJoined) eventType() string { return "player.joined" }

type PlayerUpdated struct {
	Header
	Player Player `json:"player"`
}

func (*PlayerUpdated) eventType() string { return "player.updated" }

type PlayerLeft struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*PlayerLeft) eventType() string { return "player.left" }

type PlayerKicked struct {
	Header
	Reason string `json:"reason"`
}

func (*PlayerKicked) eventType() string { return "player.kicked" }
func (*PlayerKicked) Transient() bool   { return true }

type RoomUpdated struct {
	Header
	Room RoomInfo `json:"room"`
}

func (*RoomUpdated) eventType() string { return "room.updated" }

type RoomClosed struct {
	Header
}

func (*RoomClosed) eventType() string { return "room.closed" }
func (*RoomClosed) Transient() bool   { return true }

type PlayerKick struct {
	ID ulid.ULID `json:"id"`
}

const KickReason = "The GM removed you from the room."

func (c *PlayerKick) Authorize(s *State, a Actor) error {
	if err := requireGM(a, "remove somebody from the room"); err != nil {
		return err
	}
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
	s.Players = slices.DeleteFunc(s.Players, func(q Player) bool { return q.ID == c.ID })
	s.Normalize()
	return []Emission{
		toPlayer(c.ID, &PlayerKicked{Reason: KickReason}),
		to(ToAll, &PlayerLeft{ID: c.ID}),
	}, nil
}

type PlayerJoin struct {
	Player Player `json:"player"`
}

func (c *PlayerJoin) Authorize(s *State, a Actor) error { return nil }
func (c *PlayerJoin) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p := clonePlayer(c.Player)
	p.Connected = true
	if existing := s.Player(p.ID); existing != nil {
		*existing = p
		s.Normalize()
		return []Emission{to(ToAll, &PlayerUpdated{Player: clonePlayer(p)})}, nil
	}
	s.Players = append(s.Players, p)
	s.Normalize()
	return []Emission{to(ToAll, &PlayerJoined{Player: clonePlayer(p)})}, nil
}

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
	p.Connected = c.Connected
	s.Normalize()
	return []Emission{to(ToAll, &PlayerUpdated{Player: clonePlayer(*p)})}, nil
}

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

type RoomSetLocked struct {
	Locked bool `json:"locked"`
}

func (c *RoomSetLocked) Authorize(s *State, a Actor) error { return nil }
func (c *RoomSetLocked) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	s.Room.Locked = c.Locked
	s.Normalize()
	return []Emission{to(ToAll, &RoomUpdated{Room: s.Room})}, nil
}

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

type RoomClose struct{}

func (c *RoomClose) Authorize(s *State, a Actor) error { return nil }
func (c *RoomClose) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	return []Emission{to(ToAll, &RoomClosed{})}, nil
}
func clonePlayer(p Player) Player {
	p.CharacterID = cloneID(p.CharacterID)
	return p
}
