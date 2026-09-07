package room

import (
	"encoding/json"

	"github.com/oklog/ulid/v2"
)

// Header is the three fields every event carries, and it is embedded first in
// every event type so that encoding/json flattens it to the front of the
// object. Reading a frame in devtools then starts with what it is.
type Header struct {
	// Type is the flat noun.verbed name. It is set by EncodeEvent from the
	// type's own eventType method rather than written into each literal,
	// because a struct whose Type says one thing and whose Go type is another
	// is a bug no test would catch.
	Type string `json:"type"`

	// Seq is the room's monotonic sequence, stamped by the hub. Nothing in
	// this package assigns it: an Apply does not know how many events were
	// sent before it, and sequence assignment across audiences is phase 3's
	// problem.
	Seq uint64 `json:"seq"`

	// By is the player who caused the event, absent when the server did. It is
	// what lets a client skip its own echo where skipping is wanted, and what
	// puts a name on a toast.
	By *ulid.ULID `json:"by,omitempty"`
}

func (h *Header) header() *Header { return h }

// Transient defaults to false here so that adding an event type is one method
// rather than two, and so that the answer for a new event is the safe one: a
// state event that forgot to say so is reduced, where a transient event that
// forgot would be dropped from the snapshot and lost on reconnect.
func (h *Header) Transient() bool { return false }

// Event is one thing that happened, in the past tense. It is what the server
// broadcasts and never what a client sent: the server's version may be snapped,
// resolved against the monster manual, or projected down to less than the
// commanding client asked about.
//
// THE TWO UNEXPORTED METHODS KEEP THE SET CLOSED. Every event is declared in
// this package, which is what lets the registry below be the complete list --
// and the registry being complete is what the TypeScript generator relies on to
// emit a discriminated union that the client can switch on exhaustively.
type Event interface {
	eventType() string
	header() *Header

	// Transient marks the events that never touch the reducer. They are
	// handled by an effects layer instead -- a ping fades, a drag ghost is
	// drawn and forgotten -- and they are deliberately absent from the
	// snapshot, because there is nothing about them to restore.
	Transient() bool
}

// EncodeEvent stamps the header and marshals. It is the only way an event
// becomes bytes, so there is one place that decides a frame's shape.
//
// IT MUTATES THE EVENT'S HEADER, which is safe for the reason the whole design
// is single-goroutine: one room owns its state and its emissions, and encodes
// them one at a time. An event sent to both audiences is encoded twice with the
// same seq, which writes the same three values twice.
func EncodeEvent(ev Event, seq uint64, by *ulid.ULID) ([]byte, error) {
	h := ev.header()
	h.Type = ev.eventType()
	h.Seq = seq
	h.By = by

	return json.Marshal(ev)
}

// roleView is implemented by the events whose player copy is not their GM copy.
// There are three, all of them sent to an audience that spans both roles, and
// each one carries both copies from the moment Apply built it -- see the
// comment on ForRole for why the projection happens there and not here.
type roleView interface {
	ForRole(role Role) Event
}

// ForRole is what the hub calls on every emission before encoding it. It
// returns the copy that role may see, or nil when that role may see nothing of
// this event at all.
//
// MOST EVENTS ARE ALREADY RIGHT FOR THEIR AUDIENCE and return themselves,
// because Apply is where the projection happens: Apply has the state, so it
// knows the room's hit-point setting and which pawns are on the active layer,
// and an event sent ToPlayers was built with the player's copy of the pawn in
// it. Only an event whose audience spans both roles -- ToAll, ToOthers -- has
// to carry two versions of itself, and those three implement this.
//
// NIL IS A REAL ANSWER, not an error. A move of three pawns that are all hidden
// has nothing to tell players; sending an empty list would have every player's
// renderer wake up to do nothing.
func ForRole(ev Event, role Role) Event {
	if v, ok := ev.(roleView); ok {
		return v.ForRole(role)
	}

	return ev
}

// eventTypes is every event the server can send. Like the command registry it
// exists so that one map is the authority: the generator emits from it, the
// reducer switches on the same set, and a test walks it to check that each
// type's own eventType matches the key it is filed under.
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

// ErrorEvent is the one event no Apply returns. The hub builds it from whatever
// Authorize or Apply refused with, pairs it with the correlation id off the
// frame that caused it, and sends it to that client alone.
type ErrorEvent struct {
	Header
	CID     string `json:"cid"`
	Code    string `json:"code"`
	Heading string `json:"heading"`
	Message string `json:"message"`
}

func (*ErrorEvent) eventType() string { return "error" }
func (*ErrorEvent) Transient() bool   { return true }

// NewErrorEvent turns a refusal into the frame for it. An error that is not a
// *Error -- which would be a bug in this package rather than in the client --
// becomes a generic invalid rather than leaking whatever the internal message
// happened to say.
func NewErrorEvent(cid string, err error) *ErrorEvent {
	e, ok := err.(*Error)
	if !ok {
		e = &Error{Code: CodeInvalid, Heading: "Something went wrong", Message: "The server could not carry that out."}
	}

	return &ErrorEvent{CID: cid, Code: e.Code, Heading: e.Heading, Message: e.Message}
}

// EventPrototypes is one fresh zero value of every event, by type string, for
// the generator and for the tests that walk the whole catalog.
func EventPrototypes() map[string]Event {
	out := make(map[string]Event, len(eventTypes))
	for name, build := range eventTypes {
		out[name] = build()
	}

	return out
}
