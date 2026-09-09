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
//
// SIZE IS ON THE WIRE AND EVERY OTHER STAT IS NOT, which looks arbitrary and is
// not. A monster and a character both bring a size with them, from a column, so
// this is here for the one creature that brings none: an NPC placed from a
// picture. Size is also the one of these that is structural rather than
// informational -- it decides the footprint, which decides the snapping lattice
// and the drawn radius -- so a creature placed at the wrong one is placed in
// the wrong place. Hit points and armour class are numbers a GM fills in once
// they know what the thing is, and the pawn dialog is where they do it;
// putting them here would put a stat block in a spawn dialog.
//
// AN OBJECT'S SIZE IS NOT ON THE WIRE AT ALL, and that is the same argument
// reaching the opposite answer. An object is a picture on the table and the
// picture already has a size, in the assets row that resolution reads, so there
// is nothing for a browser to say about it and nothing to be trusted or gone
// stale. The GM changes it afterwards with PawnUpdate if the picture was not
// what they wanted; they are not asked before it is down.
//
// NEITHER IS ITS ANGLE. A token goes down square and is turned afterwards, by
// the handles on the canvas or by the number in its dialog -- both of which are
// PawnUpdate. Asking a dialog for an angle before anything is on the table
// would be asking somebody to guess at a rotation they cannot yet see.
//
// It is READ BY THE HUB AND NOT BY Apply, like MonsterID and AssetID beside it.
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
	Size        Size       `json:"size,omitempty"`

	Pawn *Pawn `json:"-"`
}

// Authorize is the GM and nobody else, a player's own character included.
//
// PUTTING SOMETHING ON THE TABLE IS ONE PERSON'S JOB. A player who arrives late
// does not place their own pawn and is not asked to: the GM presses Spawn
// pawns and everybody connected who joined with a character gets one, which is
// PawnSpawnCharacters below. That keeps one pair of hands deciding what is on
// the map and where it starts, which is what a table with a screen at one end
// of it already looks like.
//
// THE ALLOWANCE WAS REMOVED RATHER THAN HIDDEN. This used to let a player spawn
// the character they joined with, and taking the menu item away would not have
// been the same thing: the socket accepts commands from whoever is connected,
// so a rule that only a button enforces is not a rule.
//
// THE LAYER IS NOT CHECKED HERE ANY MORE and does not need to be.
// requirePlayerLayer only ever refused a player naming a floor nobody is
// looking at, and a player no longer reaches this at all. That the layer
// EXISTS is still checked, by requireLayer at the top of Apply.
func (c *PawnSpawn) Authorize(_ *State, a Actor) error {
	return requireGM(a, "put something on the table")
}

func (c *PawnSpawn) Apply(s *State, _ Actor, env Env) ([]Emission, error) {
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

	return s.addPawn(p, env)
}

// PawnSpawnCharacters is the "spawn the party" button. It has no wire fields at
// all: who is at the table and which character each of them joined with are
// facts the hub holds, so it resolves the whole list and Apply places it.
//
// THE PARTY ARRIVES SHOWN, ALWAYS, and that is the one place a spawn is not
// asked. PawnSpawn carries a visibility off the wire because the dialog behind
// it has a switch: a GM sets an ambush up before the players are meant to know
// about it. This button is the opposite gesture -- it puts the people who are
// sitting at the table onto the table -- and a party that landed hidden would
// be five players told nothing happened, followed by the GM revealing each of
// them one at a time to undo a default nobody chose.
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
		p.Visible = true

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
		p.Rotation = normalizeRotation(p.Rotation)
	} else {
		p.Width, p.Height, p.Rotation = 0, 0, 0
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
		return checkObjectSize(p.Width, p.Height)
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
	ID       ulid.ULID `json:"id"`
	Name     *string   `json:"name,omitempty"`
	HP       *int      `json:"hp,omitempty"`
	MaxHP    *int      `json:"maxHp,omitempty"`
	AC       *int      `json:"ac,omitempty"`
	Size     *Size     `json:"size,omitempty"`
	Z        *int      `json:"z,omitempty"`
	Width    *int      `json:"width,omitempty"`
	Height   *int      `json:"height,omitempty"`
	Rotation *int      `json:"rotation,omitempty"`
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
			return nil, invalid("Wrong pawn", "An object is measured in pixels rather than by a creature size.")
		}
		next.Size = *c.Size
	}
	if c.Width != nil || c.Height != nil || c.Rotation != nil {
		if next.Kind != PawnObject {
			return nil, invalid("Wrong pawn", "A creature has a size rather than a rectangle at an angle.")
		}
		if c.Width != nil {
			next.Width = *c.Width
		}
		if c.Height != nil {
			next.Height = *c.Height
		}
		if c.Rotation != nil {
			next.Rotation = normalizeRotation(*c.Rotation)
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
//
// IT TAKES A LIST FOR PawnSetLayer's REASON AND SENDS ONE FOR THE SAME ONE. The
// canvas overlay hides a whole selection at once -- the eight goblins waiting
// round the corner go away together or the reveal is eight separate moments --
// and the pawn's own dialog sends a list of one rather than there being a
// second command for it. Doing it in one command is also what keeps the tracker
// to a single event: eight commands would have every player re-render the turn
// order eight times to reach the same answer.
type PawnSetVisible struct {
	IDs     []ulid.ULID `json:"ids"`
	Visible bool        `json:"visible"`
}

func (c *PawnSetVisible) Authorize(s *State, a Actor) error {
	return requireGM(a, "hide or reveal pawns")
}

func (c *PawnSetVisible) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}

	// WHETHER THE TURN ORDER CHANGED IS A DIFFERENT QUESTION FROM WHETHER A
	// PAWN APPEARED, and getting the two confused leaves a player's tracker
	// naming a creature the GM has hidden.
	//
	// THE TABLE ASKS Shown AND THE TRACKER ASKS Visible, which is the whole of
	// it. What players are SENT is the active layer's visible pawns, so a
	// goblin in the cellar is not on their table whatever its flag says --
	// while projectInitiative keeps an entry for a pawn on another floor and
	// drops one for a pawn that is hidden, deliberately: a creature that walked
	// downstairs still has a turn, and a hidden one is a creature they have not
	// met. So hiding a tracked goblin in the cellar moves nothing across the
	// shown line and still takes a line out of their turn order.
	//
	// IT IS ASKED BEFORE THE FLAGS ARE WRITTEN, because "did this change" has
	// no answer afterwards.
	tracked := false
	for _, id := range c.IDs {
		if s.Pawn(id).Visible != c.Visible && s.hasEntryFor(id) {
			tracked = true

			break
		}
	}

	before := s.shownSet()
	for _, id := range c.IDs {
		s.Pawn(id).Visible = c.Visible
	}
	s.Normalize()

	// The GM sees ordinary updates, because to them nothing appeared or
	// disappeared -- a flag changed on pawns that were on their screen before
	// and are on it after.
	var out []Emission
	for _, p := range s.Pawns {
		if slices.Contains(c.IDs, p.ID) {
			out = append(out, to(ToGM, &PawnUpdated{Pawn: clonePawn(p)}))
		}
	}

	out = append(out, s.shownTransitions(before)...)

	// The GM's tracker did not change, which is why this goes to players alone
	// -- and it goes once however many pawns were hidden.
	if tracked {
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

// ProjectedPawn is one pawn as one role may see it, or nil when that role is
// shown nothing at all. It is the only way out of this package to a single
// pawn, and it is exported for exactly one caller: the hub, answering the HTTP
// fragments that draw a pawn's panel and its stat block.
//
// IT IS PROJECTED BY CONSTRUCTION AND THAT IS THE POINT OF ITS EXISTING. The
// two-audience design rests on a hidden pawn never reaching a player's browser,
// and every socket emission honours it because Apply runs the payload through
// projectPawn on the way to ToPlayers. A fragment route is a second door into
// the same state, and one that handed back the stored pawn would be a way round
// all of it -- a player guesses a ULID, GETs the panel, and reads the hit
// points the socket was careful never to send.
//
// So clonePawn and projectPawn stay unexported and this is what crosses the
// import. A caller cannot ask for the unprojected pawn because there is nothing
// to ask; it can only forget to check for nil, which is an empty panel rather
// than a leak.
func (s *State) ProjectedPawn(id ulid.ULID, role Role) *Pawn {
	p := s.Pawn(id)
	if p == nil {
		return nil
	}

	if role == RoleGM {
		out := clonePawn(*p)

		return &out
	}

	if !s.Shown(*p) {
		return nil
	}

	out := projectPawn(clonePawn(*p), s.Table)

	return &out
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
