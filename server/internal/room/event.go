package room
import (
	"encoding/json"
	"github.com/oklog/ulid/v2"
)
type Header struct {
	Type string `json:"type"`
	Seq uint64 `json:"seq"`
	By *ulid.ULID `json:"by,omitempty"`
}
func (h *Header) header() *Header { return h }
func (h *Header) Transient() bool { return false }
type Event interface {
	eventType() string
	header() *Header
	Transient() bool
}
func EncodeEvent(ev Event, seq uint64, by *ulid.ULID) ([]byte, error) {
	h := ev.header()
	h.Type = ev.eventType()
	h.Seq = seq
	h.By = by
	return json.Marshal(ev)
}
type roleView interface {
	ForRole(role Role) Event
}
func ForRole(ev Event, role Role) Event {
	if v, ok := ev.(roleView); ok {
		return v.ForRole(role)
	}
	return ev
}
var eventTypes = map[string]func() Event{
	"snapshot":           func() Event { return &Snapshot{} },
	"room.updated":       func() Event { return &RoomUpdated{} },
	"room.closed":        func() Event { return &RoomClosed{} },
	"table.updated":      func() Event { return &TableUpdated{} },
	"player.joined":      func() Event { return &PlayerJoined{} },
	"player.updated":     func() Event { return &PlayerUpdated{} },
	"player.left":        func() Event { return &PlayerLeft{} },
	"player.kicked":      func() Event { return &PlayerKicked{} },
	"pawn.spawned":       func() Event { return &PawnSpawned{} },
	"pawn.updated":       func() Event { return &PawnUpdated{} },
	"pawn.removed":       func() Event { return &PawnRemoved{} },
	"pawn.moved":         func() Event { return &PawnMoved{} },
	"pawn.dragging":      func() Event { return &PawnDragging{} },
	"initiative.updated": func() Event { return &InitiativeUpdated{} },
	"fog.added":          func() Event { return &FogAdded{} },
	"fog.removed":        func() Event { return &FogRemoved{} },
	"fog.cleared":        func() Event { return &FogCleared{} },
	"stroke.began":       func() Event { return &StrokeBegan{} },
	"stroke.extended":    func() Event { return &StrokeExtended{} },
	"stroke.ended":       func() Event { return &StrokeEnded{} },
	"stroke.erased":      func() Event { return &StrokeErased{} },
	"stroke.cleared":     func() Event { return &StrokeCleared{} },
	"pinged":             func() Event { return &Pinged{} },
	"error":              func() Event { return &ErrorEvent{} },
}
type ErrorEvent struct {
	Header
	CID     string `json:"cid"`
	Code    string `json:"code"`
	Heading string `json:"heading"`
	Message string `json:"message"`
}
func (*ErrorEvent) eventType() string { return "error" }
func (*ErrorEvent) Transient() bool   { return true }
func NewErrorEvent(cid string, err error) *ErrorEvent {
	e, ok := err.(*Error)
	if !ok {
		e = &Error{Code: CodeInvalid, Heading: "Something went wrong", Message: "The server could not carry that out."}
	}
	return &ErrorEvent{CID: cid, Code: e.Code, Heading: e.Heading, Message: e.Message}
}
func EventPrototypes() map[string]Event {
	out := make(map[string]Event, len(eventTypes))
	for name, build := range eventTypes {
		out[name] = build()
	}
	return out
}
