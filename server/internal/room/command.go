package room

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/oklog/ulid/v2"
)

// Actor is who is asking. It is two fields because that is everything any
// authorization rule in this package needs: an id to compare against a pawn's
// owner or a stroke's author, and a role to compare against RoleGM.
//
// IT IS NOT A *Player. A player row can be absent -- the GM's own row is
// inserted by the hub like anyone else's, and a command can arrive in the
// window before it is -- and every rule here would then need a nil check that
// says nothing. The hub builds an Actor from the connection it already
// authenticated, and the connection is the authority on both fields.
type Actor struct {
	ID   ulid.ULID
	Role Role
}

// GM is the role check, spelled once so the twenty-odd Authorize methods below
// read as sentences rather than as comparisons.
func (a Actor) GM() bool { return a.Role == RoleGM }

// Env is the ambient facts an Apply needs that are neither state nor command.
// There are two, and both are here for the same reason: they are the only
// things in this package that are not a pure function of its inputs, so they
// are named, injected and therefore testable.
type Env struct {
	// NewID mints the ids Apply assigns. In production this is ulid.Make,
	// which is monotonic within a millisecond, so pawns spawned in one
	// operation sort in the order they were created. In a test it returns a
	// fixed sequence, which is what makes a golden fixture a diff rather than
	// a regeneration.
	NewID func() ulid.ULID

	// Version is the server build the room is running. It rides in the
	// snapshot event, and a client whose bundle was built from a different one
	// reloads rather than trying to speak a protocol it was not generated
	// against. Only sync.request reads it.
	Version string
}

// id is the guarded call. A zero Env is useful -- NewState in a test, an Apply
// that mints nothing -- and it should not panic, so a missing NewID falls back
// to the real one rather than to nil.
func (e Env) id() ulid.ULID {
	if e.NewID == nil {
		return ulid.Make()
	}

	return e.NewID()
}

// Command is one thing a client asked for. The split into two methods is the
// whole reason this package is testable without a socket: Authorize answers
// "may this actor do this at all" against an unmodified state, and Apply
// answers "what changes and who is told", and neither one needs a connection,
// a request or a database.
//
// AUTHORIZE NEVER MUTATES. Apply may still fail: Authorize decides on identity
// and role, Apply decides on the state it is about to change, and the two
// cannot be merged without either mutating during a check or checking twice.
//
// AUTHORIZE MAY ALSO ANSWER not_found, which the plan's sketch did not allow
// for. "May you move pawn X" has no answer when X is not there, and answering
// forbidden would tell a player that their own pawn belongs to somebody else.
type Command interface {
	Authorize(s *State, a Actor) error
	Apply(s *State, a Actor, env Env) ([]Emission, error)
}

// Audience is who an emission reaches. The hub resolves one of these to a set
// of connections; it makes no decision of its own about who sees what, because
// every such decision needs the state and the state is here.
type Audience int

const (
	// ToAll is every connected client. An event sent this way whose player
	// copy differs from the GM's carries both, and the hub asks it for the
	// one it is about to encode -- see ForRole.
	ToAll Audience = iota

	// ToGM is the owner's clients, however many tabs they have open.
	ToGM

	// ToPlayers is every client whose role is player, the actor included.
	// Echoing a command back to the client that sent it is deliberate: the
	// sender converges from the same event as everybody else rather than from
	// its own optimistic guess.
	ToPlayers

	// ToSender is the actor's own clients and nobody else.
	ToSender

	// ToOthers is everybody except the actor. Exactly one event uses it, and
	// the reason is in pawn.go.
	ToOthers

	// ToPlayer is one named player, in Emission.Player.
	ToPlayer
)

// Emission is one event and the audience it goes to. Apply returns a slice of
// these in the order they must be sent, and that order is part of the
// specification: a client that receives pawn.removed before the table.updated
// that explains it would flicker.
type Emission struct {
	Event  Event
	To     Audience
	Player ulid.ULID
}

// to is the constructor the Apply methods use, so an emission is one short call
// at the point where the reasoning about the audience is written down.
func to(a Audience, ev Event) Emission { return Emission{Event: ev, To: a} }

// toPlayer is the one-recipient form.
func toPlayer(id ulid.ULID, ev Event) Emission {
	return Emission{Event: ev, To: ToPlayer, Player: id}
}

// Error is a refusal the client can show. It is a value rather than a sentinel
// because the client renders it: the code drives behaviour (a locked room sends
// the player back to the join page, an invalid command is a bug report), and
// the heading and message go straight into the alert modal the rest of the app
// already uses.
type Error struct {
	Code    string `json:"code"`
	Heading string `json:"heading"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("room: %s: %s", e.Code, e.Message)
}

// The codes. They are strings on the wire rather than numbers because they are
// read in devtools while a new protocol is being built, which the overview
// gives as the reason the whole wire format is text.
const (
	// CodeForbidden is a real actor asking for something their role or their
	// ownership does not cover. It is never the client's bug.
	CodeForbidden = "forbidden"

	// CodeInvalid is a command that could not be true: a limit exceeded, a
	// malformed colour, an unknown type. It is always the client's bug, and
	// the client is expected to report it rather than retry.
	CodeInvalid = "invalid"

	// CodeNotFound is a command naming something that is gone. It is nobody's
	// bug -- two people deleting the same pawn is a race the table produces on
	// its own -- so the client resyncs rather than complaining.
	CodeNotFound = "not_found"

	// CodeLocked is the room refusing a join. Nothing in this package returns
	// it yet; the lock is enforced at the HTTP join in phase 1 and at the
	// socket upgrade in phase 3, and the code lives here so both spell it the
	// same way.
	CodeLocked = "locked"

	// CodeRateLimited is the hub's, for the same reason: the per-connection
	// message cap is phase 3's, and this is the word it will use.
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

// wireCommands is every command a browser may send, by its type string. The
// map is the authority for three things at once: what DecodeCommand will
// build, what the TypeScript generator emits, and what the authorization test
// iterates. Adding a command in one place and forgetting it in the other two is
// the drift this removes.
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

// hubCommands is the second registry, and the important thing about it is that
// DecodeCommand does not consult it. These are built by the hub from connection
// lifecycle events and from HTTP handlers -- a browser that sends
// {"type":"player.join","player":{"role":"gm"}} gets an unknown type back,
// which is the entire defence and it is structural rather than a check
// somebody has to remember to write.
var hubCommands = map[string]func() Command{
	"player.join":         func() Command { return &PlayerJoin{} },
	"player.setConnected": func() Command { return &PlayerSetConnected{} },
	"player.leave":        func() Command { return &PlayerLeave{} },
	"room.setLocked":      func() Command { return &RoomSetLocked{} },
	"room.setName":        func() Command { return &RoomSetName{} },
	"room.close":          func() Command { return &RoomClose{} },
}

// DecodeCommand turns one text frame into a command and the client's
// correlation id. Everything it can refuse is invalid, because every one of
// them is a client that sent something no generated client could produce.
//
// THE ENVELOPE IS READ TWICE AND THE PAYLOAD ONCE. type and cid are not fields
// of any command type -- they belong to the frame, not to the operation -- so
// they are lifted out and the rest is decoded strictly. Strictly matters: an
// unknown field is a client built against a different protocol version, and
// silently dropping it is how a pawn ends up spawning without the property the
// client thought it sent.
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

	// The payload is the frame minus the envelope. Re-marshalling a map of raw
	// messages is not the cheapest way to get there, but this runs once per
	// command from a browser and the alternative -- reflecting over each
	// command's tags to build the set of known fields -- is a second
	// implementation of what DisallowUnknownFields already does correctly.
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

// THE SHARED CHECKS. Every Authorize below is written out of these, so the
// twenty-odd of them differ only where the rule differs and a reader comparing
// two of them is comparing the rules rather than the spelling.

// requireGM refuses anybody but the room's owner. what names the operation in
// the message, because "Only the GM can do that" tells a player nothing about
// what they clicked.
func requireGM(a Actor, what string) error {
	if !a.GM() {
		return forbidden("Only the GM", "Only the GM can "+what+".")
	}

	return nil
}

// requireLayer resolves a layer id or explains that it is gone. Two GMs, or one
// GM in two tabs, can delete a layer while the other is still looking at it.
func (s *State) requireLayer(id ulid.ULID) (*Layer, error) {
	l := s.Layer(id)
	if l == nil {
		return nil, notFound("Layer gone", "That layer is no longer on the table.")
	}

	return l, nil
}

// requirePawn resolves a pawn id or explains that it is gone.
func (s *State) requirePawn(id ulid.ULID) (*Pawn, error) {
	p := s.Pawn(id)
	if p == nil {
		return nil, notFound("Pawn gone", "That pawn is no longer on the table.")
	}

	return p, nil
}

// requirePlayerLayer is the rule that keeps a player's commands on the floor
// they are looking at. A player who could name another layer could ping into a
// room the party has not found, or begin a stroke on a map they cannot see, and
// in both cases the giveaway is that the GM's screen changes.
//
// It is a no-op for the GM, who is the one person entitled to work on a layer
// nobody is looking at.
func (s *State) requirePlayerLayer(a Actor, layer ulid.ULID) error {
	if a.GM() || layer == s.Table.ActiveLayer {
		return nil
	}

	return forbidden("Wrong layer", "Players can only act on the layer the table is showing.")
}

// shownSet records which pawns players can currently see. Four commands change
// that set as a side effect rather than as their purpose -- hiding a pawn,
// moving one to another floor, changing which floor is active, deleting a floor
// -- and each of them takes one of these before and compares after.
func (s *State) shownSet() map[ulid.ULID]bool {
	set := make(map[ulid.ULID]bool, len(s.Pawns))
	for _, p := range s.Pawns {
		if s.Shown(p) {
			set[p.ID] = true
		}
	}

	return set
}

// shownTransitions turns the difference between a shownSet and the present into
// the events players need: a pawn that stopped being shown is removed from
// their table, and one that started is spawned onto it.
//
// REMOVALS COME FIRST, and the iteration is over the pawn slice, which
// Normalize keeps sorted. Both matter for the same reason: these emissions are
// compared byte for byte in the golden fixtures, and an order that depended on
// map iteration would be a test that fails one run in six.
func (s *State) shownTransitions(before map[ulid.ULID]bool) []Emission {
	var removed, spawned []Emission

	for _, p := range s.Pawns {
		switch {
		case before[p.ID] && !s.Shown(p):
			removed = append(removed, to(ToPlayers, &PawnRemoved{ID: p.ID}))
		case !before[p.ID] && s.Shown(p):
			spawned = append(spawned, to(ToPlayers, &PawnSpawned{Pawn: projectPawn(clonePawn(p), s.Table)}))
		}
	}

	return append(removed, spawned...)
}

// The event payload clones. An emission is a value the hub holds until it has
// encoded it, and a test holds one for the length of a scenario; sharing a
// slice with the live state would let a later command edit an earlier event.

// CloneTable is a deep copy of the table: the layer slice and every map
// reference in it, so a caller may hold one while the room goes on changing.
//
// IT IS EXPORTED FOR THE HUB, which hands a copy out of the room goroutine to
// the HTTP handler drawing the layer manager. Everything else in the protocol
// clones inside this package, but a table crossing a goroutine boundary is the
// one place where sharing the backing array would be a data race rather than
// merely a surprise.
func CloneTable(t Table) Table {
	// The source is held before the destination is allocated. Ranging over the
	// field after reassigning it would range over the fresh, empty slice --
	// which is a copy that silently produces zeroes.
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

// cloneSlice copies a slice and never hands back a nil one.
//
// THAT IS THE WHOLE REASON IT EXISTS, and it is not a style preference. The
// obvious spelling, append([]T(nil), src...), returns NIL when src is empty --
// so an event carrying a pawn with no conditions marshals `"conditions": null`
// while the state's copy of the same pawn, which Normalize has been over,
// marshals `[]`. The generated TypeScript declares that field as an array, and
// a client that had to check for null on a field the types say is always there
// is a client with a bug waiting in whichever branch nobody wrote.
//
// Normalize does exactly this for the state. These are for the entities that
// travel in events, which Normalize never sees -- an Apply clones the pawn it
// is about to append and emits that, so the clone is where the guarantee has to
// be made or it is not made at all.
func cloneSlice[T any](src []T) []T {
	return append(make([]T, 0, len(src)), src...)
}

// WireCommandPrototypes is one fresh zero value of every command a browser may
// send, by type string. It is what the TypeScript generator reflects over and
// what the authorization test iterates, and it is a function returning fresh
// values rather than the map itself so that neither of those can accidentally
// hand out a shared instance or edit the registry.
func WireCommandPrototypes() map[string]Command {
	return prototypes(wireCommands)
}

// HubCommandPrototypes is the same for the commands only the server builds.
// The generator does not emit these -- a client that had a type for
// player.join would look like a client that could send one.
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
