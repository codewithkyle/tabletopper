package room

import (
	"encoding/json"

	"github.com/oklog/ulid/v2"
)

type Header struct {
	Type string     `json:"type"`
	Seq  uint64     `json:"seq"`
	By   *ulid.ULID `json:"by,omitempty"`
}

func (h *Header) header() *Header { return h }

type Event interface {
	eventType() string
	header() *Header
}
type Transient interface {
	Event
	transient()
}
type Kind struct {
	Type string `json:"type"`
}

func (k *Kind) kind() *Kind { return k }

type Change interface {
	changeType() string
	kind() *Kind
}

func EncodeEvent(ev Event, seq uint64, by *ulid.ULID) ([]byte, error) {
	h := ev.header()
	h.Type = ev.eventType()
	h.Seq = seq
	h.By = by
	return json.Marshal(ev)
}

var eventTypes = map[string]func() Event{
	"changes":       func() Event { return &Changes{} },
	"snapshot":      func() Event { return &Snapshot{} },
	"room.closed":   func() Event { return &RoomClosed{} },
	"player.kicked": func() Event { return &PlayerKicked{} },
	"pawn.dragging": func() Event { return &PawnDragging{} },
	"pinged":        func() Event { return &Pinged{} },
	"error":         func() Event { return &ErrorEvent{} },
}

type ErrorEvent struct {
	Header
	CID     string `json:"cid"`
	Code    string `json:"code"`
	Heading string `json:"heading"`
	Message string `json:"message"`
}

func (*ErrorEvent) eventType() string { return "error" }
func (*ErrorEvent) transient()        {}
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
