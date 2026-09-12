package room

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/oklog/ulid/v2"
)

type Actor struct {
	ID   ulid.ULID
	Role Role
}

func (a Actor) GM() bool { return a.Role == RoleGM }

type Env struct {
	NewID   func() ulid.ULID
	Version string
}

func (e Env) id() ulid.ULID {
	if e.NewID == nil {
		return ulid.Make()
	}
	return e.NewID()
}

type Command interface {
	Authorize(s *State, a Actor) error
	Apply(s *State, a Actor, env Env) ([]Signal, error)
}
type Audience int

const (
	ToAll Audience = iota
	ToSender
	ToOthers
	ToPlayer
)

type Signal struct {
	Event  Transient
	To     Audience
	Player ulid.ULID
}

func signal(a Audience, ev Transient) Signal { return Signal{Event: ev, To: a} }

type Error struct {
	Code    string `json:"code"`
	Heading string `json:"heading"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("room: %s: %s", e.Code, e.Message)
}

const (
	CodeForbidden   = "forbidden"
	CodeInvalid     = "invalid"
	CodeNotFound    = "not_found"
	CodeLocked      = "locked"
	CodeRateLimited = "rate_limited"
)

func forbidden(heading, message string) error {
	return &Error{Code: CodeForbidden, Heading: heading, Message: message}
}
func invalid(heading, message string) error {
	return &Error{Code: CodeInvalid, Heading: heading, Message: message}
}
func notFound(heading, message string) error {
	return &Error{Code: CodeNotFound, Heading: heading, Message: message}
}

var wireCommands = map[string]func() Command{
	"table.addLayer":       func() Command { return &TableAddLayer{} },
	"table.removeLayer":    func() Command { return &TableRemoveLayer{} },
	"table.renameLayer":    func() Command { return &TableRenameLayer{} },
	"table.moveLayer":      func() Command { return &TableMoveLayer{} },
	"table.setLayerMap":    func() Command { return &TableSetLayerMap{} },
	"table.clearLayerMap":  func() Command { return &TableClearLayerMap{} },
	"table.setActiveLayer": func() Command { return &TableSetActiveLayer{} },
	"table.setGrid":        func() Command { return &TableSetGrid{} },
	"table.setOptions":     func() Command { return &TableSetOptions{} },
	"table.clear":          func() Command { return &TableClear{} },
	"pawn.spawn":           func() Command { return &PawnSpawn{} },
	"pawn.spawnCharacters": func() Command { return &PawnSpawnCharacters{} },
	"pawn.move":            func() Command { return &PawnMove{} },
	"pawn.drag":            func() Command { return &PawnDrag{} },
	"pawn.update":          func() Command { return &PawnUpdate{} },
	"pawn.setConditions":   func() Command { return &PawnSetConditions{} },
	"pawn.setVisible":      func() Command { return &PawnSetVisible{} },
	"pawn.setLayer":        func() Command { return &PawnSetLayer{} },
	"pawn.remove":          func() Command { return &PawnRemove{} },
	"initiative.set":       func() Command { return &InitiativeSet{} },
	"initiative.sync":      func() Command { return &InitiativeSync{} },
	"initiative.next":      func() Command { return &InitiativeNext{} },
	"initiative.clear":     func() Command { return &InitiativeClear{} },
	"initiative.activate":  func() Command { return &InitiativeActivate{} },
	"initiative.remove":    func() Command { return &InitiativeRemove{} },
	"initiative.reorder":   func() Command { return &InitiativeReorder{} },
	"initiative.add":       func() Command { return &InitiativeAdd{} },
	"fog.setEnabled":       func() Command { return &FogSetEnabled{} },
	"fog.setPrefill":       func() Command { return &FogSetPrefill{} },
	"fog.add":              func() Command { return &FogAdd{} },
	"fog.remove":           func() Command { return &FogRemove{} },
	"fog.clear":            func() Command { return &FogClear{} },
	"stroke.begin":         func() Command { return &StrokeBegin{} },
	"stroke.extend":        func() Command { return &StrokeExtend{} },
	"stroke.end":           func() Command { return &StrokeEnd{} },
	"stroke.erase":         func() Command { return &StrokeErase{} },
	"stroke.clear":         func() Command { return &StrokeClear{} },
	"ping":                 func() Command { return &Ping{} },
	"player.kick":          func() Command { return &PlayerKick{} },
	"sync.request":         func() Command { return &SyncRequest{} },
}
var hubCommands = map[string]func() Command{
	"player.join":         func() Command { return &PlayerJoin{} },
	"player.setConnected": func() Command { return &PlayerSetConnected{} },
	"player.leave":        func() Command { return &PlayerLeave{} },
	"room.setLocked":      func() Command { return &RoomSetLocked{} },
	"room.setName":        func() Command { return &RoomSetName{} },
	"room.close":          func() Command { return &RoomClose{} },
}

func DecodeCommand(b []byte) (Command, string, error) {
	var env struct {
		Type string `json:"type"`
		CID  string `json:"cid"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, "", invalid("Bad message", "The server could not read that message.")
	}
	build, ok := wireCommands[env.Type]
	if !ok {
		return nil, env.CID, invalid("Unknown command", fmt.Sprintf("The server does not know a command called %q.", env.Type))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, env.CID, invalid("Bad message", "The server could not read that message.")
	}
	delete(fields, "type")
	delete(fields, "cid")
	payload, err := json.Marshal(fields)
	if err != nil {
		return nil, env.CID, invalid("Bad message", "The server could not read that message.")
	}
	cmd := build()
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cmd); err != nil {
		return nil, env.CID, invalid("Bad command", fmt.Sprintf("The %s command was not shaped the way the server expects.", env.Type))
	}
	return cmd, env.CID, nil
}
func requireGM(a Actor, what string) error {
	if !a.GM() {
		return forbidden("Only the GM", "Only the GM can "+what+".")
	}
	return nil
}
func (s *State) requireLayer(id ulid.ULID) (*Layer, error) {
	l := s.Layer(id)
	if l == nil {
		return nil, notFound("Layer gone", "That layer is no longer on the table.")
	}
	return l, nil
}
func (s *State) requirePawn(id ulid.ULID) (*Pawn, error) {
	p := s.Pawn(id)
	if p == nil {
		return nil, notFound("Pawn gone", "That pawn is no longer on the table.")
	}
	return p, nil
}
func (s *State) requirePlayerLayer(a Actor, layer ulid.ULID) error {
	if a.GM() || layer == s.Table.ActiveLayer {
		return nil
	}
	return forbidden("Wrong layer", "Players can only act on the layer the table is showing.")
}
func CloneTable(t Table) Table {
	src := t.Layers
	t.Layers = make([]Layer, len(src))
	for i, l := range src {
		l.Map = cloneRef(l.Map)
		t.Layers[i] = l
	}
	return t
}
func cloneInitiative(i Initiative) Initiative {
	src := i.Entries
	i.Active = cloneID(i.Active)
	i.Entries = make([]InitiativeEntry, len(src))
	for n, e := range src {
		e.PawnIDs = cloneSlice(e.PawnIDs)
		i.Entries[n] = e
	}
	return i
}
func cloneShape(f FogShape) FogShape {
	f.Points = cloneSlice(f.Points)
	return f
}
func cloneStroke(st Stroke) Stroke {
	st.Points = cloneSlice(st.Points)
	return st
}
func cloneSlice[T any](src []T) []T {
	return append(make([]T, 0, len(src)), src...)
}
func WireCommandPrototypes() map[string]Command {
	return prototypes(wireCommands)
}
func HubCommandPrototypes() map[string]Command {
	return prototypes(hubCommands)
}
func prototypes(from map[string]func() Command) map[string]Command {
	out := make(map[string]Command, len(from))
	for name, build := range from {
		out[name] = build()
	}
	return out
}
