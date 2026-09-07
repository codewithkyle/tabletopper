package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE PAWN FAMILY, which is most of what a session does.
//
// FOUR OF THE FIVE EVENTS CARRY A WHOLE PAWN. An update to a goblin's hit
// points sends the goblin, not the hit points. The old protocol had eight
// per-field pawn events and each one was a separate chance for the client's
// copy to drift from the server's; one event carrying the entity converges with
// a single assignment by id, and it is a few hundred bytes.
//
// THE FIFTH IS THE EXCEPTION AND IT IS DELIBERATE. pawn.moved and
// pawn.dragging carry ids and coordinates, because they are the only messages
// that fire while a hand is moving. They are also the only non-idempotent
// events in the protocol, which is the price, and the reason there are two of
// them rather than a general delta mechanism.
//
// EVERY EMISSION TO PLAYERS IS ALREADY PROJECTED. Apply has the state, so it
// knows the room's hit-point setting and which floor is active; the pawn put
// into a ToPlayers event has been through projectPawn before it gets there, and
// the pawn put into a ToGM event has not. There is no flag on the wire that a
// client is trusted to respect.

// PawnSpawned puts a pawn on a client's table. Players receive one only for a
// pawn that is shown to them, so it doubles as "this became visible".
type PawnSpawned struct {
	Header
	Pawn Pawn `json:"pawn"`
}

func (*PawnSpawned) eventType() string { return "pawn.spawned" }

// PawnUpdated replaces a pawn wholesale.
type PawnUpdated struct {
	Header
	Pawn Pawn `json:"pawn"`
}

func (*PawnUpdated) eventType() string { return "pawn.updated" }

// PawnRemoved takes a pawn off a client's table. To the GM it always means the
// pawn is gone; to a player it may also mean the pawn was hidden or walked up
// a staircase, and the player has no way to tell those apart, which is the
// point.
type PawnRemoved struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*PawnRemoved) eventType() string { return "pawn.removed" }

// PawnMoved is the committed positions of everything that moved.
//
// IT CARRIES BOTH AUDIENCES' LISTS. It goes to everybody, and a group that
// includes a hidden pawn must reach players without it; the filtered list is
// built by Apply, which is the only place that knows what is hidden, and
// ForRole hands out whichever one the hub is about to encode.
type PawnMoved struct {
	Header
	Pawns []PawnPosition `json:"pawns"`

	shown []PawnPosition
}

func (*PawnMoved) eventType() string { return "pawn.moved" }

func (e *PawnMoved) ForRole(role Role) Event {
	if role == RoleGM {
		return e
	}
	if len(e.shown) == 0 {
		return nil
	}

	c := *e
	c.Pawns = e.shown

	return &c
}

// PawnDragging is a preview. It changes no state, is never reduced, and goes to
// everybody except the person whose hand is on the mouse -- they are already
// drawing it, at their pointer's rate rather than at the network's.
type PawnDragging struct {
	Header
	Pawns []PawnPosition `json:"pawns"`

	shown []PawnPosition
}

func (*PawnDragging) eventType() string { return "pawn.dragging" }
func (*PawnDragging) Transient() bool   { return true }

func (e *PawnDragging) ForRole(role Role) Event {
	if role == RoleGM {
		return e
	}
	if len(e.shown) == 0 {
		return nil
	}

	c := *e
	c.Pawns = e.shown

	return &c
}

// PawnSpawn puts one pawn on the table.
//
// IT IS A RESOLVED COMMAND. The wire carries a reference -- a monster id, a
// character id, or a token asset -- and the hub turns that into a whole pawn
// with a name, a picture and a stat line before Apply runs, because all three
// of those are rows in the database and this package has none. Apply then owns
// the fields that are the table's rather than the source's: where it stands,
// which floor it is on, whether players can see it.
type PawnSpawn struct {
	Kind        PawnKind   `json:"kind"`
	Layer       ulid.ULID  `json:"layer"`
	X           int        `json:"x"`
	Y           int        `json:"y"`
	Visible     bool       `json:"visible"`
	MonsterID   *ulid.ULID `json:"monsterId,omitempty"`
	CharacterID *ulid.ULID `json:"characterId,omitempty"`
	AssetID     *ulid.ULID `json:"assetId,omitempty"`
	Name        string     `json:"name,omitempty"`
	FootprintW  int        `json:"footprintW,omitempty"`
	FootprintH  int        `json:"footprintH,omitempty"`

	Pawn *Pawn `json:"-"`
}

// Authorize lets a player spawn exactly one thing: their own character, on the
// floor everybody is looking at. That is the "I joined late" case, and it is
// the only reason a player needs this command at all -- everything else on the
// table is the GM's to place.
func (c *PawnSpawn) Authorize(s *State, a Actor) error {
	if err := s.requirePlayerLayer(a, c.Layer); err != nil {
		return err
	}
	if a.GM() {
		return nil
	}

	if c.Kind != PawnPlayer {
		return forbidden("Only the GM", "Only the GM can put that on the table.")
	}

	me := s.Player(a.ID)
	if me == nil || me.CharacterID == nil || c.CharacterID == nil || *me.CharacterID != *c.CharacterID {
		return forbidden("Not your character", "You can only place the character you joined with.")
	}

	return nil
}

func (c *PawnSpawn) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if !c.Kind.Valid() {
		return nil, invalid("Bad pawn", "That is not a kind of pawn.")
	}
	if c.Pawn == nil {
		return nil, invalid("Nothing to place", "The server could not work out what to put on the table.")
	}

	p := clonePawn(*c.Pawn)
	p.Kind = c.Kind
	p.LayerID = c.Layer
	p.X, p.Y = c.X, c.Y
	p.Visible = c.Visible
	if c.Name != "" {
		p.Name = c.Name
	}
	if c.Kind == PawnObject {
		p.FootprintW, p.FootprintH = c.FootprintW, c.FootprintH
	}

	// A player's pawn belongs to the player who placed it whatever the hub
	// resolved, so that the answer to "may I move this" cannot be arranged by
	// the client that asked for it.
	if !a.GM() {
		p.OwnerID = &a.ID
	}

	return s.addPawn(p, env)
}

// PawnSpawnCharacters is the "spawn the party" button. It has no wire fields at
// all: who is at the table and which character each of them joined with are
// facts the hub holds, so it resolves the whole list and Apply places it.
type PawnSpawnCharacters struct {
	Pawns []Pawn `json:"-"`
}

func (c *PawnSpawnCharacters) Authorize(s *State, a Actor) error {
	return requireGM(a, "spawn the party")
}

func (c *PawnSpawnCharacters) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if c.Pawns == nil {
		return nil, invalid("Nothing to place", "The server could not work out who is at the table.")
	}

	var out []Emission
	for _, p := range c.Pawns {
		p = clonePawn(p)
		p.Kind = PawnPlayer

		em, err := s.addPawn(p, env)
		if err != nil {
			return nil, err
		}
		out = append(out, em...)
	}

	return out, nil
}

// addPawn is the tail both spawns share: validate, give it an id, snap it, put
// it on top, and tell the two audiences.
func (s *State) addPawn(p Pawn, env Env) ([]Emission, error) {
	if len(s.Pawns) >= PawnsMax {
		return nil, invalid("Table full", "There are already as many pawns on the table as a room can hold.")
	}
	if s.Layer(p.LayerID) == nil {
		return nil, notFound("Layer gone", "That layer is no longer on the table.")
	}
	if err := checkPawn(p); err != nil {
		return nil, err
	}

	p.ID = env.id()
	p.HPBand = nil
	if p.Kind == PawnObject {
		p.Size = ""
		p.Conditions = nil
	} else {
		p.FootprintW, p.FootprintH = 0, 0
	}
	clampHP(&p)

	p.X, p.Y = snapPawn(s.Table.Grid, p, p.X, p.Y)
	p.Z = s.maxZ() + 1

	s.Pawns = append(s.Pawns, p)
	s.Normalize()

	out := []Emission{to(ToGM, &PawnSpawned{Pawn: clonePawn(p)})}
	if s.Shown(p) {
		out = append(out, to(ToPlayers, &PawnSpawned{Pawn: projectPawn(clonePawn(p), s.Table)}))
	}

	return out, nil
}

// checkPawn validates the fields a pawn carries whatever put it there, so a
// resolved pawn from the hub is held to the same limits as one a browser
// described.
func checkPawn(p Pawn) error {
	if err := checkName("pawn", p.Name); err != nil {
		return err
	}
	if err := checkCoord("position", p.X); err != nil {
		return err
	}
	if err := checkCoord("position", p.Y); err != nil {
		return err
	}
	if err := checkHP(p.HP, p.MaxHP); err != nil {
		return err
	}
	if err := checkAC(p.AC); err != nil {
		return err
	}

	if p.Kind == PawnObject {
		return checkFootprint(p.FootprintW, p.FootprintH)
	}

	if !p.Size.Valid() {
		return invalid("Bad pawn", "That is not a creature size.")
	}
	if len(p.Conditions) > ConditionsMax {
		return invalid("Too many conditions", "A pawn can carry at most 16 conditions.")
	}
	for _, cond := range p.Conditions {
		if err := checkCondition(cond); err != nil {
			return err
		}
	}

	return nil
}

// PawnMove commits a move. The anchor is the pawn under the pointer; the others
// are whatever else was selected, and they ride.
type PawnMove struct {
	Anchor ulid.ULID   `json:"anchor"`
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Others []ulid.ULID `json:"others"`
}

func (c *PawnMove) Authorize(s *State, a Actor) error {
	return s.requireControl(a, c.Anchor, c.Others)
}

func (c *PawnMove) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	ids, err := s.selection(c.Anchor, c.Others)
	if err != nil {
		return nil, err
	}

	anchor := s.Pawn(c.Anchor)
	nx, ny := snapPawn(s.Table.Grid, *anchor, c.X, c.Y)

	// ONE DELTA, TAKEN FROM THE ANCHOR'S COMMITTED POSITION AND APPLIED TO
	// EVERYTHING. The riders are not snapped: a wagon with three people sitting
	// on it must arrive with them sitting in the same three spots, and snapping
	// each one independently would shuffle them into the wagon's cells.
	dx, dy := nx-anchor.X, ny-anchor.Y

	// All or nothing. Every new position is computed and checked before any
	// pawn is written, so a group that would push one member off the map moves
	// nobody rather than moving everybody but them.
	moved := make([]PawnPosition, 0, len(ids))
	for _, id := range ids {
		p := s.Pawn(id)
		x, y := p.X+dx, p.Y+dy
		if err := checkCoord("position", x); err != nil {
			return nil, err
		}
		if err := checkCoord("position", y); err != nil {
			return nil, err
		}
		moved = append(moved, PawnPosition{ID: id, X: x, Y: y})
	}

	for _, m := range moved {
		p := s.Pawn(m.ID)
		p.X, p.Y = m.X, m.Y
	}
	s.Normalize()

	return []Emission{to(ToAll, &PawnMoved{Pawns: moved, shown: s.shownPositions(moved)})}, nil
}

// PawnDrag is the preview of a move that has not happened. It carries the same
// three fields and changes nothing.
type PawnDrag struct {
	Anchor ulid.ULID   `json:"anchor"`
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Others []ulid.ULID `json:"others"`
}

func (c *PawnDrag) Authorize(s *State, a Actor) error {
	return s.requireControl(a, c.Anchor, c.Others)
}

func (c *PawnDrag) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	ids, err := s.selection(c.Anchor, c.Others)
	if err != nil {
		return nil, err
	}

	// A DRAG IS NEVER SNAPPED, on purpose. The dragging client snaps its own
	// ghost for its own display, and the delta here is computed from the raw
	// pointer, so a client with snapping off and one with it on both see the
	// other's ghost land exactly where that person's screen shows it.
	anchor := s.Pawn(c.Anchor)
	dx, dy := c.X-anchor.X, c.Y-anchor.Y

	at := make([]PawnPosition, 0, len(ids))
	for _, id := range ids {
		p := s.Pawn(id)
		at = append(at, PawnPosition{ID: id, X: p.X + dx, Y: p.Y + dy})
	}

	// ToOthers is the one use of that audience in the protocol, and it is the
	// reason the audience exists: everybody but the sender, spanning both
	// roles, which no pair of single-role emissions can express.
	return []Emission{{
		Event: &PawnDragging{Pawns: at, shown: s.shownPositions(at)},
		To:    ToOthers,
	}}, nil
}

// requireControl is the authority rule move and drag share: the GM may move
// anything, and a player may move what they own and can see.
//
// A MISSING PAWN IS not_found RATHER THAN forbidden. Answering forbidden would
// tell a player that a pawn they own belongs to somebody else, which is both
// false and confusing; two people deleting the same goblin is an ordinary race
// and the client's answer to it is to resync.
func (s *State) requireControl(a Actor, anchor ulid.ULID, others []ulid.ULID) error {
	for _, id := range append([]ulid.ULID{anchor}, others...) {
		p, err := s.requirePawn(id)
		if err != nil {
			return err
		}
		if a.GM() {
			continue
		}
		if p.OwnerID == nil || *p.OwnerID != a.ID {
			return forbidden("Not your pawn", "You can only move pawns you own.")
		}
		if !s.Shown(*p) {
			return forbidden("Not your pawn", "You can only move pawns you own.")
		}
	}

	return nil
}

// selection is the anchor plus the others, bounded, de-duplicated and with
// every id resolved. De-duplication matters because a client that puts the
// anchor in its own selection would otherwise move it twice, by twice the
// delta.
func (s *State) selection(anchor ulid.ULID, others []ulid.ULID) ([]ulid.ULID, error) {
	if err := checkSelection(len(others) + 1); err != nil {
		return nil, err
	}

	ids := []ulid.ULID{anchor}
	seen := map[ulid.ULID]bool{anchor: true}
	for _, id := range others {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	for _, id := range ids {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}

	return ids, nil
}

// shownPositions is the players' copy of a position list: the ones they can
// see, in the same order. An empty result is what makes ForRole answer nil, so
// a move of nothing but hidden pawns is not sent to them at all.
func (s *State) shownPositions(all []PawnPosition) []PawnPosition {
	out := make([]PawnPosition, 0, len(all))
	for _, m := range all {
		if p := s.Pawn(m.ID); p != nil && s.Shown(*p) {
			out = append(out, m)
		}
	}

	return out
}

// PawnUpdate changes the plain fields of one pawn. Every field is a pointer, so
// absent means "leave it" -- which also means there is no way to clear hit
// points back to unknown once they are set. That is deliberate: a GM sets a
// monster's hit points and then changes them, and an "unset" that could be
// reached by a client sending null is a way to lose a stat line by accident.
type PawnUpdate struct {
	ID         ulid.ULID `json:"id"`
	Name       *string   `json:"name,omitempty"`
	HP         *int      `json:"hp,omitempty"`
	MaxHP      *int      `json:"maxHp,omitempty"`
	AC         *int      `json:"ac,omitempty"`
	Size       *Size     `json:"size,omitempty"`
	Z          *int      `json:"z,omitempty"`
	FootprintW *int      `json:"footprintW,omitempty"`
	FootprintH *int      `json:"footprintH,omitempty"`
}

func (c *PawnUpdate) Authorize(s *State, a Actor) error {
	return s.requireOwner(a, c.ID, "change that pawn")
}

func (c *PawnUpdate) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p, err := s.requirePawn(c.ID)
	if err != nil {
		return nil, err
	}

	next := clonePawn(*p)
	if c.Name != nil {
		next.Name = *c.Name
	}
	if c.HP != nil {
		next.HP = cloneInt(c.HP)
	}
	if c.MaxHP != nil {
		next.MaxHP = cloneInt(c.MaxHP)
	}
	if c.AC != nil {
		next.AC = cloneInt(c.AC)
	}
	if c.Z != nil {
		next.Z = *c.Z
	}

	if c.Size != nil {
		if next.Kind == PawnObject {
			return nil, invalid("Wrong pawn", "An object has a footprint rather than a size.")
		}
		next.Size = *c.Size
	}
	if c.FootprintW != nil || c.FootprintH != nil {
		if next.Kind != PawnObject {
			return nil, invalid("Wrong pawn", "A creature has a size rather than a footprint.")
		}
		if c.FootprintW != nil {
			next.FootprintW = *c.FootprintW
		}
		if c.FootprintH != nil {
			next.FootprintH = *c.FootprintH
		}
	}

	if err := checkPawn(next); err != nil {
		return nil, err
	}
	clampHP(&next)

	*p = next
	s.Normalize()

	return s.pawnUpdated(c.ID), nil
}

// PawnSetConditions replaces a pawn's whole condition list, which is the
// singleton rule applied to a field: the client sends the chips it wants the
// pawn to have and the server does not have to reason about which one moved.
type PawnSetConditions struct {
	ID         ulid.ULID   `json:"id"`
	Conditions []Condition `json:"conditions"`
}

func (c *PawnSetConditions) Authorize(s *State, a Actor) error {
	return s.requireOwner(a, c.ID, "change that pawn")
}

func (c *PawnSetConditions) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p, err := s.requirePawn(c.ID)
	if err != nil {
		return nil, err
	}
	if p.Kind == PawnObject {
		return nil, invalid("Wrong pawn", "An object cannot be poisoned.")
	}
	if len(c.Conditions) > ConditionsMax {
		return nil, invalid("Too many conditions", "A pawn can carry at most 16 conditions.")
	}

	conditions := make([]Condition, 0, len(c.Conditions))
	for _, cond := range c.Conditions {
		if err := checkCondition(cond); err != nil {
			return nil, err
		}

		// A chip the client has just invented arrives without an id, and the
		// server mints it. One that is being kept arrives with the id it
		// already had, so that a duration ticking down does not look like a
		// different condition every round.
		if cond.ID.Compare(ulid.ULID{}) == 0 {
			cond.ID = env.id()
		}
		conditions = append(conditions, cond)
	}

	p.Conditions = conditions
	s.Normalize()

	return s.pawnUpdated(c.ID), nil
}

// PawnSetVisible is the GM's hide and reveal. It is the command the whole
// two-audience design exists for.
type PawnSetVisible struct {
	ID      ulid.ULID `json:"id"`
	Visible bool      `json:"visible"`
}

func (c *PawnSetVisible) Authorize(s *State, a Actor) error {
	return requireGM(a, "hide or reveal a pawn")
}

func (c *PawnSetVisible) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	p, err := s.requirePawn(c.ID)
	if err != nil {
		return nil, err
	}

	was := s.Shown(*p)
	p.Visible = c.Visible
	s.Normalize()

	p = s.Pawn(c.ID)
	now := s.Shown(*p)

	// The GM sees an ordinary update, because to them nothing appeared or
	// disappeared -- a flag changed on a pawn that was on their screen before
	// and is on it after.
	out := []Emission{to(ToGM, &PawnUpdated{Pawn: clonePawn(*p)})}
	if was == now {
		return out, nil
	}

	if now {
		out = append(out, to(ToPlayers, &PawnSpawned{Pawn: projectPawn(clonePawn(*p), s.Table)}))
	} else {
		out = append(out, to(ToPlayers, &PawnRemoved{ID: c.ID}))
	}

	// A HIDDEN PAWN'S TURN IS NOT IN THE PLAYERS' TRACKER, so revealing or
	// hiding one changes a second thing on their screen. The GM's tracker did
	// not change, which is why this goes to players alone.
	if s.hasEntryFor(c.ID) {
		out = append(out, to(ToPlayers, &InitiativeUpdated{Initiative: projectInitiative(s)}))
	}

	return out, nil
}

// PawnSetLayer moves pawns between floors.
//
// IT IS GM-ONLY AND THAT IS NOT AN ARBITRARY CHOICE. A player who moved their
// own pawn to another layer would be holding a pawn they can no longer see,
// which is exactly the state the projection exists to prevent, and there is no
// sensible thing for their client to draw afterwards.
type PawnSetLayer struct {
	IDs   []ulid.ULID `json:"ids"`
	Layer ulid.ULID   `json:"layer"`
}

func (c *PawnSetLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "move pawns between layers")
}

func (c *PawnSetLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}

	before := s.shownSet()
	for _, id := range c.IDs {
		s.Pawn(id).LayerID = c.Layer
	}
	s.Normalize()

	var out []Emission
	for _, p := range s.Pawns {
		if slices.Contains(c.IDs, p.ID) {
			out = append(out, to(ToGM, &PawnUpdated{Pawn: clonePawn(p)}))
		}
	}

	return append(out, s.shownTransitions(before)...), nil
}

// PawnRemove takes pawns off the table for good.
type PawnRemove struct {
	IDs []ulid.ULID `json:"ids"`
}

func (c *PawnRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove pawns")
}

func (c *PawnRemove) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}

	var out []Emission
	var entriesChanged bool

	for _, id := range c.IDs {
		p := s.Pawn(id)
		if p == nil {
			continue
		}

		out = append(out, to(ToGM, &PawnRemoved{ID: id}))
		if s.Shown(*p) {
			out = append(out, to(ToPlayers, &PawnRemoved{ID: id}))
		}
		if s.dropEntriesFor(id) {
			entriesChanged = true
		}

		s.Pawns = slices.DeleteFunc(s.Pawns, func(q Pawn) bool { return q.ID == id })
	}

	s.Normalize()

	// ONE TRACKER EVENT FOR THE WHOLE COMMAND, not one per pawn. The tracker is
	// a singleton: sending it eight times for eight deleted goblins would have
	// every client re-render the turn order eight times to reach the same
	// answer.
	if entriesChanged {
		out = append(out, initiativeUpdated(s)...)
	}

	return out, nil
}

// requireOwner is the authority rule the two per-pawn edits share: the GM, or
// the player the pawn belongs to.
func (s *State) requireOwner(a Actor, id ulid.ULID, what string) error {
	p, err := s.requirePawn(id)
	if err != nil {
		return err
	}
	if a.GM() {
		return nil
	}
	if p.OwnerID == nil || *p.OwnerID != a.ID || !s.Shown(*p) {
		return forbidden("Not your pawn", "Only the GM can "+what+".")
	}

	return nil
}

// pawnUpdated is the pair of emissions an edit to one pawn produces: the whole
// pawn to the GM, and the projected pawn to players when they can see it.
func (s *State) pawnUpdated(id ulid.ULID) []Emission {
	p := s.Pawn(id)
	if p == nil {
		return nil
	}

	out := []Emission{to(ToGM, &PawnUpdated{Pawn: clonePawn(*p)})}
	if s.Shown(*p) {
		out = append(out, to(ToPlayers, &PawnUpdated{Pawn: projectPawn(clonePawn(*p), s.Table)}))
	}

	return out
}
