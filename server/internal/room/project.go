package room

import "github.com/oklog/ulid/v2"

func (s *State) Project(role Role) State {
	c := s.Clone()
	if role == RoleGM {
		return c
	}
	pawns := make([]Pawn, 0, len(c.Pawns))
	for _, p := range c.Pawns {
		if !s.Shown(p) {
			continue
		}
		pawns = append(pawns, projectPawn(p, c.Table))
	}
	c.Pawns = pawns
	c.Initiative = projectInitiative(s)
	c.Normalize()
	return c
}
func projectPawn(p Pawn, t Table) Pawn {
	if p.Kind != PawnMonster && p.Kind != PawnNPC {
		return p
	}
	if t.PawnLabels == LabelsFull {
		return p
	}
	p.AC = nil
	p.HPBand = nil
	if t.PawnLabels == LabelsDefault {
		p.HPBand = hpBand(p.HP, p.MaxHP)
	}
	return p
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
func ProjectSignal(s *State, sig Signal, role Role) Event {
	drag, ok := sig.Event.(*PawnDragging)
	if !ok || role == RoleGM {
		return sig.Event
	}
	shown := make([]PawnPosition, 0, len(drag.Pawns))
	for _, at := range drag.Pawns {
		if p := s.Pawn(at.ID); p != nil && s.Shown(*p) {
			shown = append(shown, at)
		}
	}
	if len(shown) == 0 {
		return nil
	}
	c := *drag
	c.Pawns = shown
	return &c
}
func Health(p Pawn) *HPBand {
	if p.HP != nil {
		return hpBand(p.HP, p.MaxHP)
	}
	return p.HPBand
}
func Dead(p Pawn) bool {
	b := Health(p)
	return b != nil && *b == BandDead
}
func hpBand(hp, maxHP *int) *HPBand {
	if hp == nil || maxHP == nil || *maxHP < 1 {
		return nil
	}
	band := BandHealthy
	switch {
	case *hp <= 0:
		band = BandDead
	case *hp*20 <= *maxHP:
		band = BandNearDeath
	case *hp*4 <= *maxHP:
		band = BandVeryBloody
	case *hp*2 <= *maxHP:
		band = BandBloody
	case *hp*4 <= *maxHP*3:
		band = BandBruised
	}
	return &band
}
