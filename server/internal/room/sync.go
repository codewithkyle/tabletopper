package room

import "github.com/oklog/ulid/v2"

type Snapshot struct {
	Header
	State   State       `json:"state"`
	You     SnapshotYou `json:"you"`
	Version string      `json:"version"`
	Now     int64       `json:"now"`
}

func (*Snapshot) eventType() string { return "snapshot" }
func (*Snapshot) transient()        {}

type SnapshotYou struct {
	ID   ulid.ULID `json:"id"`
	Role Role      `json:"role"`
}
type SyncRequest struct{}

func (c *SyncRequest) preview()                          {}
func (c *SyncRequest) Authorize(s *State, a Actor) error { return nil }
func (c *SyncRequest) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	return []Signal{signal(ToSender, &Snapshot{
		State:   s.Project(a.Role),
		You:     SnapshotYou{ID: a.ID, Role: a.Role},
		Version: env.Version,
		Now:     env.now(),
	})}, nil
}
