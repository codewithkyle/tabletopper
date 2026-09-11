package room
import (
	"slices"
	"github.com/oklog/ulid/v2"
)
type InitiativeUpdated struct {
	Header
	Initiative Initiative `json:"initiative"`
	player *Initiative
}
func (*InitiativeUpdated) eventType() string { return "initiative.updated" }
func (e *InitiativeUpdated) ForRole(role Role) Event {
	if role == RoleGM || e.player == nil {
		return e
	}
	c := *e
	c.Initiative = *e.player
	return &c
}
func initiativeUpdated(s *State) []Emission {
	player := projectInitiative(s)
	return []Emission{to(ToAll, &InitiativeUpdated{
		Initiative: cloneInitiative(s.Initiative),
		player:     &player,
	})}
}
func projectInitiative(s *State) Initiative {
	in := cloneInitiative(s.Initiative)
	entries := make([]InitiativeEntry, 0, len(in.Entries))
	for _, e := range in.Entries {
		if len(e.PawnIDs) > 0 {
			kept := make([]ulid.ULID, 0, len(e.PawnIDs))
			for _, id := range e.PawnIDs {
				if p := s.Pawn(id); p != nil && p.Visible {
					kept = append(kept, id)
				}
			}
			if len(kept) == 0 {
				continue
			}
			e.PawnIDs = kept
		}
		entries = append(entries, e)
	}
	in.Entries = entries
	if in.Active != nil && !hasEntry(entries, *in.Active) {
		in.Active = nil
	}
	return in
}
func (s *State) ProjectedInitiative(role Role) (Initiative, map[ulid.ULID]Pawn) {
	in := cloneInitiative(s.Initiative)
	if role != RoleGM {
		in = projectInitiative(s)
	}
	pawns := map[ulid.ULID]Pawn{}
	for _, e := range in.Entries {
		for _, id := range e.PawnIDs {
			if _, done := pawns[id]; done {
				continue
			}
			p := s.Pawn(id)
			if p == nil {
				continue
			}
			if role == RoleGM {
				pawns[id] = clonePawn(*p)
				continue
			}
			if !p.Visible {
				continue
			}
			pawns[id] = projectPawn(clonePawn(*p), s.Table)
		}
	}
	return in, pawns
}
func (s *State) hasEntryFor(pawn ulid.ULID) bool {
	for _, e := range s.Initiative.Entries {
		if slices.Contains(e.PawnIDs, pawn) {
			return true
		}
	}
	return false
}
func (s *State) dropEntriesFor(pawn ulid.ULID) bool {
	if !s.hasEntryFor(pawn) {
		return false
	}
	return s.dropMembers(func(id ulid.ULID) bool { return id == pawn })
}
func (s *State) dropMembers(gone func(ulid.ULID) bool) bool {
	entries, active, changed := dropped(s.Initiative.Entries, s.Initiative.Active, gone)
	s.Initiative.Entries = entries
	s.Initiative.Active = active
	if len(entries) == 0 {
		if s.Initiative.Active != nil || s.Initiative.Round != 0 {
			changed = true
		}
		s.Initiative.Active = nil
		s.Initiative.Round = 0
	}
	return changed
}
func dropped(entries []InitiativeEntry, active *ulid.ULID, gone func(ulid.ULID) bool) ([]InitiativeEntry, *ulid.ULID, bool) {
	empties := func(e InitiativeEntry) bool {
		if len(e.PawnIDs) == 0 {
			return false
		}
		for _, id := range e.PawnIDs {
			if !gone(id) {
				return false
			}
		}
		return true
	}
	changed := false
	at := -1
	if active != nil {
		at = slices.IndexFunc(entries, func(e InitiativeEntry) bool { return e.ID == *active })
	}
	if at >= 0 && empties(entries[at]) {
		var successor *ulid.ULID
		n := len(entries)
		for i := 1; i < n; i++ {
			e := entries[(at+i)%n]
			if !empties(e) {
				id := e.ID
				successor = &id
				break
			}
		}
		active = successor
		changed = true
	}
	kept := make([]InitiativeEntry, 0, len(entries))
	for _, e := range entries {
		if empties(e) {
			changed = true
			continue
		}
		members := make([]ulid.ULID, 0, len(e.PawnIDs))
		for _, id := range e.PawnIDs {
			if gone(id) {
				changed = true
				continue
			}
			members = append(members, id)
		}
		e.PawnIDs = members
		kept = append(kept, e)
	}
	return kept, active, changed
}
type InitiativeSet struct {
	Entries []InitiativeEntry `json:"entries"`
	Active  *ulid.ULID        `json:"active"`
}
func (c *InitiativeSet) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the initiative order")
}
func (c *InitiativeSet) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(c.Entries) > InitiativeMax {
		return nil, invalid("Too many entries", "The tracker holds at most 200 entries.")
	}
	entries := make([]InitiativeEntry, 0, len(c.Entries))
	seen := map[ulid.ULID]bool{}
	claimed := map[ulid.ULID]bool{}
	for _, e := range c.Entries {
		if err := checkRequiredName("tracker entry", e.Name); err != nil {
			return nil, err
		}
		for _, id := range e.PawnIDs {
			if _, err := s.requirePawn(id); err != nil {
				return nil, err
			}
			if claimed[id] {
				return nil, invalid("Duplicate pawn", "The same pawn is in the initiative order twice.")
			}
			claimed[id] = true
		}
		if e.ID.Compare(ulid.ULID{}) == 0 {
			e.ID = env.id()
		}
		if seen[e.ID] {
			return nil, invalid("Duplicate entry", "The same tracker entry appears twice.")
		}
		seen[e.ID] = true
		entries = append(entries, e)
	}
	if c.Active != nil && !hasEntry(entries, *c.Active) {
		return nil, invalid("Bad turn", "The active turn is not one of the entries.")
	}
	s.Initiative.Entries = entries
	s.Initiative.Active = cloneID(c.Active)
	s.Normalize()
	return initiativeUpdated(s), nil
}
type InitiativeNext struct{}
func (c *InitiativeNext) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}
	if s.Initiative.Active != nil {
		if e := s.entry(*s.Initiative.Active); e != nil {
			for _, id := range e.PawnIDs {
				if p := s.Pawn(id); p != nil && p.OwnerID != nil && *p.OwnerID == a.ID {
					return nil
				}
			}
		}
	}
	return forbidden("Not your turn", "Only the GM, or whoever's turn it is, can advance the tracker.")
}
func (c *InitiativeNext) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(s.Initiative.Entries) == 0 {
		return nil, invalid("Nothing to advance", "There is nothing in the initiative tracker.")
	}
	n := len(s.Initiative.Entries)
	from := -1
	if s.Initiative.Active != nil {
		from = slices.IndexFunc(s.Initiative.Entries, func(e InitiativeEntry) bool {
			return e.ID == *s.Initiative.Active
		})
	}
	var next int
	switch {
	case from < 0:
		next = 0
		for i := 0; i < n; i++ {
			if !s.skips(s.Initiative.Entries[i]) {
				next = i
				break
			}
		}
	default:
		steps := 1
		for ; steps <= n; steps++ {
			if !s.skips(s.Initiative.Entries[(from+steps)%n]) {
				break
			}
		}
		if steps > n {
			steps = 1
		}
		next = (from + steps) % n
		if from+steps >= n {
			s.Initiative.Round++
		}
	}
	touched := map[ulid.ULID]bool{}
	if from >= 0 {
		s.tick(s.Initiative.Entries[from], ClearEnd, touched)
	}
	s.tick(s.Initiative.Entries[next], ClearStart, touched)
	active := s.Initiative.Entries[next].ID
	s.Initiative.Active = &active
	s.Normalize()
	out := initiativeUpdated(s)
	for _, p := range s.Pawns {
		if touched[p.ID] {
			out = append(out, s.pawnUpdated(p.ID)...)
		}
	}
	return out, nil
}
func (s *State) skips(e InitiativeEntry) bool {
	found := false
	for _, id := range e.PawnIDs {
		p := s.Pawn(id)
		if p == nil {
			continue
		}
		found = true
		if p.Kind == PawnPlayer || !Dead(*p) {
			return false
		}
	}
	return found
}
func (s *State) tick(e InitiativeEntry, when ClearTrigger, touched map[ulid.ULID]bool) {
	for _, id := range e.PawnIDs {
		p := s.Pawn(id)
		if p == nil {
			continue
		}
		kept := make([]Condition, 0, len(p.Conditions))
		changed := false
		for _, cond := range p.Conditions {
			if cond.Clear != when || cond.Duration < 0 {
				kept = append(kept, cond)
				continue
			}
			changed = true
			cond.Duration--
			if cond.Duration > 0 {
				kept = append(kept, cond)
			}
		}
		if !changed {
			continue
		}
		p.Conditions = kept
		touched[p.ID] = true
	}
}
func (s *State) entry(id ulid.ULID) *InitiativeEntry {
	for i := range s.Initiative.Entries {
		if s.Initiative.Entries[i].ID == id {
			return &s.Initiative.Entries[i]
		}
	}
	return nil
}
type InitiativeSync struct{}
func (c *InitiativeSync) Authorize(s *State, a Actor) error {
	return requireGM(a, "sync the initiative tracker")
}
func (c *InitiativeSync) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	entries, active, _ := dropped(s.Initiative.Entries, s.Initiative.Active, func(id ulid.ULID) bool {
		p := s.Pawn(id)
		return p == nil || (p.Kind != PawnPlayer && Dead(*p))
	})
	floors := map[ulid.ULID]bool{}
	for _, p := range s.Pawns {
		if p.Kind == PawnPlayer && p.Visible {
			floors[p.LayerID] = true
		}
	}
	already := map[ulid.ULID]bool{}
	for _, e := range entries {
		for _, id := range e.PawnIDs {
			already[id] = true
		}
	}
	wanted := func(p Pawn) bool {
		return p.Visible && floors[p.LayerID] && !already[p.ID]
	}
	for _, p := range s.Pawns {
		if p.Kind == PawnPlayer && wanted(p) {
			entries = s.enlist(entries, p, env)
		}
	}
	for _, p := range s.Pawns {
		if p.Kind != PawnMonster && p.Kind != PawnNPC {
			continue
		}
		if wanted(p) && !Dead(p) {
			entries = s.enlist(entries, p, env)
		}
	}
	if len(entries) > InitiativeMax {
		return nil, invalid("Too many entries", "The tracker holds at most 200 entries.")
	}
	if len(entries) == 0 {
		return nil, invalid("Nothing to sync",
			"There is nobody on a floor a player is standing on, so there is no order to build. Spawn the party first.")
	}
	if active != nil && !hasEntry(entries, *active) {
		active = nil
	}
	s.Initiative.Entries = entries
	s.Initiative.Active = active
	s.Normalize()
	return initiativeUpdated(s), nil
}
func (s *State) enlist(entries []InitiativeEntry, p Pawn, env Env) []InitiativeEntry {
	if s.Table.InitiativeGrouping != GroupIndividual && p.Kind == PawnMonster {
		key := MonsterKey(p)
		for i, e := range entries {
			if s.groupKey(e) == key {
				entries[i].PawnIDs = append(entries[i].PawnIDs, p.ID)
				return entries
			}
		}
	}
	return append(entries, InitiativeEntry{
		ID:      env.id(),
		PawnIDs: []ulid.ULID{p.ID},
		Name:    p.Name,
	})
}
func (s *State) groupKey(e InitiativeEntry) string { return GroupKey(e, s.Pawn) }
func GroupKey(e InitiativeEntry, pawn func(ulid.ULID) *Pawn) string {
	for _, id := range e.PawnIDs {
		p := pawn(id)
		if p == nil {
			continue
		}
		if p.Kind != PawnMonster {
			return ""
		}
		return MonsterKey(*p)
	}
	return ""
}
func MonsterKey(p Pawn) string {
	if p.MonsterID != nil {
		return "id:" + p.MonsterID.String()
	}
	return "name:" + p.Name + "\x00" + p.Image
}
type InitiativeClear struct{}
func (c *InitiativeClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the initiative tracker")
}
func (c *InitiativeClear) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	s.Initiative = Initiative{}
	s.Normalize()
	return initiativeUpdated(s), nil
}
type InitiativeActivate struct {
	Entry ulid.ULID `json:"entry"`
}
func (c *InitiativeActivate) Authorize(s *State, a Actor) error {
	return requireGM(a, "change whose turn it is")
}
func (c *InitiativeActivate) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if s.entry(c.Entry) == nil {
		return nil, notFound("Entry gone", "That line is no longer in the initiative tracker.")
	}
	id := c.Entry
	s.Initiative.Active = &id
	s.Normalize()
	return initiativeUpdated(s), nil
}
type InitiativeRemove struct {
	Entry ulid.ULID `json:"entry"`
}
func (c *InitiativeRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove an entry from the initiative tracker")
}
func (c *InitiativeRemove) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	entries := s.Initiative.Entries
	at := slices.IndexFunc(entries, func(e InitiativeEntry) bool { return e.ID == c.Entry })
	if at < 0 {
		return nil, notFound("Entry gone", "That line is no longer in the initiative tracker.")
	}
	active := s.Initiative.Active
	if active != nil && *active == c.Entry {
		active = nil
		if n := len(entries); n > 1 {
			next := entries[(at+1)%n].ID
			active = &next
		}
	}
	s.Initiative.Entries = slices.Delete(slices.Clone(entries), at, at+1)
	s.Initiative.Active = active
	s.Normalize()
	return initiativeUpdated(s), nil
}
type InitiativeReorder struct {
	IDs []ulid.ULID `json:"ids"`
}
func (c *InitiativeReorder) Authorize(s *State, a Actor) error {
	return requireGM(a, "reorder the initiative tracker")
}
func (c *InitiativeReorder) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(c.IDs) != len(s.Initiative.Entries) {
		return nil, invalid("Order out of date", "The tracker changed while you were dragging. Try again.")
	}
	byID := make(map[ulid.ULID]InitiativeEntry, len(s.Initiative.Entries))
	for _, e := range s.Initiative.Entries {
		byID[e.ID] = e
	}
	entries := make([]InitiativeEntry, 0, len(c.IDs))
	for _, id := range c.IDs {
		e, found := byID[id]
		if !found {
			return nil, invalid("Order out of date", "The tracker changed while you were dragging. Try again.")
		}
		delete(byID, id)
		entries = append(entries, e)
	}
	s.Initiative.Entries = entries
	s.Normalize()
	return initiativeUpdated(s), nil
}
type InitiativeAdd struct {
	Name string     `json:"name"`
	Pawn *ulid.ULID `json:"pawn"`
}
func (c *InitiativeAdd) Authorize(s *State, a Actor) error {
	return requireGM(a, "add an entry to the initiative tracker")
}
func (c *InitiativeAdd) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if (c.Name == "") == (c.Pawn == nil) {
		return nil, invalid("Bad entry", "An entry is a name or a pawn, and not both.")
	}
	if len(s.Initiative.Entries) >= InitiativeMax {
		return nil, invalid("Too many entries", "The tracker holds at most 200 entries.")
	}
	entries := slices.Clone(s.Initiative.Entries)
	if c.Pawn == nil {
		if err := checkRequiredName("tracker entry", c.Name); err != nil {
			return nil, err
		}
		entries = append(entries, InitiativeEntry{ID: env.id(), Name: c.Name})
	} else {
		p, err := s.requirePawn(*c.Pawn)
		if err != nil {
			return nil, err
		}
		if s.hasEntryFor(p.ID) {
			return nil, invalid("Already in the order", "That creature already has a turn in the initiative tracker.")
		}
		entries = s.enlist(entries, *p, env)
	}
	s.Initiative.Entries = entries
	s.Normalize()
	return initiativeUpdated(s), nil
}
