package room

import "github.com/oklog/ulid/v2"

// Snapshot is the whole room, projected for one audience. It is the first
// message a client receives after connecting and the answer to sync.request,
// and those are the same message on purpose: a reconnect after a deploy, a
// client that noticed a gap in the sequence, and a fresh page load all end up
// in exactly the same state by exactly the same path.
//
// THERE IS NO REPLAY LOG BEHIND IT. Room state is tens of kilobytes, so
// catching a client up by sending it everything is cheaper to run and very much
// cheaper to reason about than a partial catch-up that has to be correct at
// every offset.
type Snapshot struct {
	Header
	State State `json:"state"`

	// You is who the receiver is, from the server's point of view. A client
	// cannot work its own role out -- it knows its user id, but whether that id
	// owns this room is a fact about a database row -- and every "may I" the
	// interface asks before offering a control reads it from here.
	You SnapshotYou `json:"you"`

	// Version is the server build. A client whose bundle was generated against
	// a different protocol reloads rather than speaking one it does not know,
	// which is what turns a mid-session deploy into a flicker instead of a
	// table full of subtly wrong tables.
	Version string `json:"version"`
}

func (*Snapshot) eventType() string { return "snapshot" }

// SnapshotYou is the receiver's own identity, sent to them alone.
type SnapshotYou struct {
	ID   ulid.ULID `json:"id"`
	Role Role      `json:"role"`
}

// SyncRequest asks for the whole state again. A client sends it when it notices
// a gap in the sequence, and after that it is holding the server's truth rather
// than its own guess at it.
type SyncRequest struct{}

// Authorize lets anybody resync. Refusing would leave a client stuck with a
// state it already knows is wrong, and the answer is projected for the asker's
// role anyway -- a player asking for a resync gets a player's room.
func (c *SyncRequest) Authorize(s *State, a Actor) error { return nil }

func (c *SyncRequest) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	// ToSender rather than ToAll: this is the one event whose content is
	// specific to one connection's role and identity, and the projection has
	// already happened here rather than in the hub.
	return []Emission{to(ToSender, &Snapshot{
		State:   s.Project(a.Role),
		You:     SnapshotYou{ID: a.ID, Role: a.Role},
		Version: env.Version,
	})}, nil
}
