package room

type SheetVitals struct {
	HP    int  `json:"hp"`
	MaxHP int  `json:"maxHp"`
	AC    int  `json:"ac"`
	Size  Size `json:"size"`
}

func PawnVitals(p Pawn) (SheetVitals, bool) {
	if p.Kind != PawnPlayer || p.CharacterID == nil {
		return SheetVitals{}, false
	}
	if p.HP == nil || p.MaxHP == nil || p.AC == nil || !p.Size.Valid() {
		return SheetVitals{}, false
	}
	return SheetVitals{HP: *p.HP, MaxHP: *p.MaxHP, AC: *p.AC, Size: p.Size}, true
}

type CharacterSync struct{ Info CharacterInfo }

func (c *CharacterSync) Authorize(s *State, a Actor) error { return nil }
func (c *CharacterSync) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	seat := s.seatFor(c.Info.ID)
	if seat != nil {
		seat.CharacterName = c.Info.Name
	}
	p := s.pawnFor(c.Info.ID)
	if p == nil {
		s.Normalize()
		return nil, nil
	}
	want := characterPawn(c.Info, seat)
	hp, maxHP, ac := clampVitals(*want.HP, *want.MaxHP, *want.AC)
	next := clonePawn(*p)
	next.Name = want.Name
	next.Image = want.Image
	next.Size = want.Size
	next.HP, next.MaxHP, next.AC = &hp, &maxHP, &ac
	if err := checkPawn(next); err != nil {
		return nil, err
	}
	*p = next
	s.Normalize()
	return nil, nil
}
func clampVitals(hp, maxHP, ac int) (int, int, int) {
	maxHP = min(max(maxHP, 1), HPLimit)
	return min(max(hp, 0), maxHP), maxHP, min(max(ac, 0), ACLimit)
}
