package room

import (
	"context"
	"slices"

	"github.com/oklog/ulid/v2"
)

const (
	npcHP = 1
	npcAC = 10
)

type PawnsUpserted struct {
	Kind
	Pawns []Pawn `json:"pawns"`
}

func (*PawnsUpserted) changeType() string { return "pawns.upserted" }

type PawnsRemoved struct {
	Kind
	IDs []ulid.ULID `json:"ids"`
}

func (*PawnsRemoved) changeType() string { return "pawns.removed" }

type PawnsMoved struct {
	Kind
	Pawns []PawnPosition `json:"pawns"`
}

func (*PawnsMoved) changeType() string { return "pawns.moved" }

type PawnDragging struct {
	Header
	Pawns []PawnPosition `json:"pawns"`
}

func (*PawnDragging) eventType() string { return "pawn.dragging" }
func (*PawnDragging) transient()        {}

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
	HP          *int       `json:"hp,omitempty"`
	MaxHP       *int       `json:"maxHp,omitempty"`
	AC          *int       `json:"ac,omitempty"`
	Pawn        *Pawn      `json:"-"`
}

func (c *PawnSpawn) Authorize(_ *State, a Actor) error {
	return requireGM(a, "put something on the table")
}
func (c *PawnSpawn) Resolve(ctx context.Context, lib Library, s *State) error {
	switch c.Kind {
	case PawnMonster:
		return c.resolveMonster(ctx, lib)
	case PawnNPC:
		return c.resolveNPC(ctx, lib)
	case PawnObject:
		return c.resolveObject(ctx, lib, s)
	case PawnPlayer:
		return c.resolveCharacter(ctx, lib, s)
	}
	return invalid("Bad pawn", "That is not a kind of pawn.")
}
func (c *PawnSpawn) resolveMonster(ctx context.Context, lib Library) error {
	if c.MonsterID == nil {
		return invalid("Nothing to place", "That spawn named no monster.")
	}
	info, err := lib.Monster(ctx, *c.MonsterID)
	if err != nil {
		return err
	}
	hp, ac := info.HP, info.AC
	c.Pawn = &Pawn{
		Name:      info.Name,
		Image:     info.Image,
		Size:      info.Size,
		HP:        &hp,
		MaxHP:     &hp,
		AC:        &ac,
		MonsterID: c.MonsterID,
	}
	return nil
}
func (c *PawnSpawn) resolveNPC(ctx context.Context, lib Library) error {
	picture, err := c.picture(ctx, lib, PictureAvatar)
	if err != nil {
		return err
	}
	name := spawnName(c.Name, picture.Name)
	if name == "" {
		return invalid("Nothing to place", "That spawn named nothing to place.")
	}
	hp, maxHP, ac := npcHP, npcHP, npcAC
	if c.MaxHP != nil {
		maxHP = *c.MaxHP
		hp = maxHP
	}
	if c.HP != nil {
		hp = *c.HP
	}
	if c.AC != nil {
		ac = *c.AC
	}
	c.Pawn = &Pawn{
		Name:  name,
		Image: picture.Image,
		Size:  CreatureSize(string(c.Size)),
		HP:    &hp,
		MaxHP: &maxHP,
		AC:    &ac,
	}
	return nil
}
func (c *PawnSpawn) resolveObject(ctx context.Context, lib Library, s *State) error {
	if c.AssetID == nil {
		return invalid("Nothing to place", "An object needs a picture from your library.")
	}
	picture, err := c.picture(ctx, lib, PictureToken)
	if err != nil {
		return err
	}
	name := spawnName(c.Name, picture.Name)
	if name == "" {
		return invalid("Nothing to place", "That spawn named nothing to place.")
	}
	width, height := picture.Width, picture.Height
	if width == 0 || height == 0 {
		width = max(s.Table.Grid.CellSize, 1)
		height = width
	}
	c.Pawn = &Pawn{Name: name, Image: picture.Image, Width: width, Height: height}
	return nil
}
func (c *PawnSpawn) picture(ctx context.Context, lib Library, kind PictureKind) (PictureInfo, error) {
	if c.AssetID == nil {
		return PictureInfo{}, nil
	}
	return lib.Picture(ctx, *c.AssetID, kind)
}
func (c *PawnSpawn) resolveCharacter(ctx context.Context, lib Library, s *State) error {
	if c.CharacterID == nil {
		return invalid("Nothing to place", "That spawn named no character.")
	}
	seat := s.seatFor(*c.CharacterID)
	if seat == nil {
		return notFound("Character gone", "Nobody at this table joined with that character.")
	}
	info, err := lib.Character(ctx, *c.CharacterID)
	if err != nil {
		return err
	}
	pawn := characterPawn(info, seat)
	c.Pawn = &pawn
	return nil
}
func (c *PawnSpawn) Apply(s *State, _ Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if !c.Kind.Valid() {
		return nil, invalid("Bad pawn", "That is not a kind of pawn.")
	}
	if c.Pawn == nil {
		return nil, invalid("Nothing to place", "The server could not work out what to put on the table.")
	}
	if c.Kind == PawnPlayer && c.CharacterID != nil && s.seatFor(*c.CharacterID) == nil {
		return nil, notFound("Character gone", "Nobody at this table joined with that character.")
	}
	p := clonePawn(*c.Pawn)
	p.Kind = c.Kind
	p.LayerID = c.Layer
	p.X, p.Y = c.X, c.Y
	p.Visible = c.Visible
	if c.Name != "" {
		p.Name = c.Name
	}
	return nil, s.addPawn(p, env)
}

type PawnSpawnCharacters struct {
	Pawns []Pawn `json:"-"`
}

func (c *PawnSpawnCharacters) Authorize(s *State, a Actor) error {
	return requireGM(a, "spawn the party")
}
func (c *PawnSpawnCharacters) Resolve(ctx context.Context, lib Library, s *State) error {
	seats := s.unplacedSeats()
	if len(seats) == 0 {
		return invalid("Nobody to place", "Everybody connected with a character already has a pawn on the table.")
	}
	centreX, centreY := 0, 0
	if layer := s.Layer(s.Table.ActiveLayer); layer != nil {
		switch {
		case layer.PartyStart != nil:
			centreX, centreY = layer.PartyStart.X, layer.PartyStart.Y
		case layer.Map != nil:
			centreX, centreY = layer.Map.Width/2, layer.Map.Height/2
		}
	}
	cell := max(s.Table.Grid.CellSize, 1)
	pawns := make([]Pawn, 0, len(seats))
	for i, seat := range seats {
		info, err := lib.Character(ctx, *seat.CharacterID)
		if err != nil {
			if gone(err) {
				continue
			}
			return err
		}
		p := characterPawn(info, &seats[i])
		p.LayerID = s.Table.ActiveLayer
		p.X = centreX + (2*i-(len(seats)-1))*cell/2
		p.Y = centreY
		pawns = append(pawns, p)
	}
	c.Pawns = pawns
	return nil
}
func (c *PawnSpawnCharacters) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if c.Pawns == nil {
		return nil, invalid("Nothing to place", "The server could not work out who is at the table.")
	}
	for _, p := range c.Pawns {
		if p.CharacterID == nil {
			continue
		}
		if s.seatFor(*p.CharacterID) == nil || s.pawnFor(*p.CharacterID) != nil {
			continue
		}
		p = clonePawn(p)
		p.Kind = PawnPlayer
		p.Visible = true
		if err := s.addPawn(p, env); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
func (s *State) addPawn(p Pawn, env Env) error {
	if len(s.Pawns) >= PawnsMax {
		return invalid("Table full", "There are already as many pawns on the table as a room can hold.")
	}
	if s.Layer(p.LayerID) == nil {
		return notFound("Layer gone", "That layer is no longer on the table.")
	}
	if err := checkPawn(p); err != nil {
		return err
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
	return nil
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
func (c *PawnMove) Apply(s *State, a Actor, env Env) ([]Signal, error) {
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
	return nil, nil
}

type PawnDrag struct {
	Anchor ulid.ULID   `json:"anchor"`
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Others []ulid.ULID `json:"others"`
}

func (c *PawnDrag) CoalesceKey() string { return "pawn.drag:" + c.Anchor.String() }
func (c *PawnDrag) preview()            {}
func (c *PawnDrag) Authorize(s *State, a Actor) error {
	return s.requireControl(a, c.Anchor, c.Others)
}
func (c *PawnDrag) Apply(s *State, a Actor, env Env) ([]Signal, error) {
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
	return []Signal{signal(ToOthers, &PawnDragging{Pawns: at})}, nil
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
func (c *PawnUpdate) Apply(s *State, a Actor, env Env) ([]Signal, error) {
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
	return nil, nil
}

type PawnSetConditions struct {
	ID         ulid.ULID   `json:"id"`
	Conditions []Condition `json:"conditions"`
}

func (c *PawnSetConditions) Authorize(s *State, a Actor) error {
	return s.requireOwner(a, c.ID, "change that pawn")
}
func (c *PawnSetConditions) Apply(s *State, a Actor, env Env) ([]Signal, error) {
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
	return nil, nil
}

type PawnSetVisible struct {
	IDs     []ulid.ULID `json:"ids"`
	Visible bool        `json:"visible"`
}

func (c *PawnSetVisible) Authorize(s *State, a Actor) error {
	return requireGM(a, "hide or reveal pawns")
}
func (c *PawnSetVisible) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}
	for _, id := range c.IDs {
		s.Pawn(id).Visible = c.Visible
	}
	s.Normalize()
	return nil, nil
}

type PawnSetLayer struct {
	IDs   []ulid.ULID `json:"ids"`
	Layer ulid.ULID   `json:"layer"`
}

func (c *PawnSetLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "move pawns between layers")
}
func (c *PawnSetLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
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
	for _, id := range c.IDs {
		s.Pawn(id).LayerID = c.Layer
	}
	s.Normalize()
	return nil, nil
}

type PawnRemove struct {
	IDs []ulid.ULID `json:"ids"`
}

func (c *PawnRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove pawns")
}
func (c *PawnRemove) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		if _, err := s.requirePawn(id); err != nil {
			return nil, err
		}
	}
	for _, id := range c.IDs {
		if s.Pawn(id) == nil {
			continue
		}
		s.dropEntriesFor(id)
		s.Pawns = slices.DeleteFunc(s.Pawns, func(q Pawn) bool { return q.ID == id })
	}
	s.Normalize()
	return nil, nil
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
