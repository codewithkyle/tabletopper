package room
import (
	"slices"
	"github.com/oklog/ulid/v2"
)
type PawnSpawned struct {
	Header
	Pawn Pawn `json:"pawn"`
}
func (*PawnSpawned) eventType() string { return "pawn.spawned" }
type PawnUpdated struct {
	Header
	Pawn Pawn `json:"pawn"`
}
func (*PawnUpdated) eventType() string { return "pawn.updated" }
type PawnRemoved struct {
	Header
	ID ulid.ULID `json:"id"`
}
func (*PawnRemoved) eventType() string { return "pawn.removed" }
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
	HP    *int `json:"hp,omitempty"`
	MaxHP *int `json:"maxHp,omitempty"`
	AC    *int `json:"ac,omitempty"`
	Pawn *Pawn `json:"-"`
}
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
	dx, dy := nx-anchor.X, ny-anchor.Y
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
	anchor := s.Pawn(c.Anchor)
	dx, dy := c.X-anchor.X, c.Y-anchor.Y
	at := make([]PawnPosition, 0, len(ids))
	for _, id := range ids {
		p := s.Pawn(id)
		at = append(at, PawnPosition{ID: id, X: p.X + dx, Y: p.Y + dy})
	}
	return []Emission{{
		Event: &PawnDragging{Pawns: at, shown: s.shownPositions(at)},
		To:    ToOthers,
	}}, nil
}
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
func (s *State) shownPositions(all []PawnPosition) []PawnPosition {
	out := make([]PawnPosition, 0, len(all))
	for _, m := range all {
		if p := s.Pawn(m.ID); p != nil && s.Shown(*p) {
			out = append(out, m)
		}
	}
	return out
}
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
		if cond.ID.Compare(ulid.ULID{}) == 0 {
			cond.ID = env.id()
		}
		conditions = append(conditions, cond)
	}
	p.Conditions = conditions
	s.Normalize()
	return s.pawnUpdated(c.ID), nil
}
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
	var out []Emission
	for _, p := range s.Pawns {
		if slices.Contains(c.IDs, p.ID) {
			out = append(out, to(ToGM, &PawnUpdated{Pawn: clonePawn(p)}))
		}
	}
	out = append(out, s.shownTransitions(before)...)
	if tracked {
		out = append(out, to(ToPlayers, &InitiativeUpdated{Initiative: projectInitiative(s)}))
	}
	return out, nil
}
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
	if entriesChanged {
		out = append(out, initiativeUpdated(s)...)
	}
	return out, nil
}
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
