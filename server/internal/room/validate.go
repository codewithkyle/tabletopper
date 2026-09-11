package room

import (
	"fmt"
	"strings"
	"unicode/utf8"
)











const (
	
	
	
	NameLimit = 128

	
	
	ConditionNameLimit = 64

	
	
	
	
	
	CoordLimit = 1_000_000

	
	
	HPLimit = 9_999
	ACLimit = 99

	
	
	
	CellSizeMin = 8
	CellSizeMax = 512

	
	
	
	
	
	
	
	
	
	
	
	
	StrokeWidthMax = 24

	
	
	
	
	StrokeChunkMax = 512

	
	
	StrokePointsMax = 20_000

	
	
	
	
	StrokesMax   = 5_000
	FogShapesMax = 2_000

	
	
	FogPointsMax = 2_000

	
	
	
	
	
	
	
	
	
	
	
	
	
	StrokePointsBudget = 200_000
	FogPointsBudget    = 200_000
	PlayerStrokeShare  = 4

	
	
	PawnsMax = 1_000

	
	
	ConditionsMax = 16

	
	
	
	SelectionMax = 200

	
	
	
	
	ObjectPixelsMax = 8_192

	
	
	InitiativeMax = 200

	
	LayersMax = 20
)



const (
	DefaultCellSize    = 64
	DefaultFeetPerCell = 5

	
	
	
	DefaultGridColor = "#000000FF"
)



const FeetPerCellMax = 1_000



func checkName(what, s string) error {
	if utf8.RuneCountInString(s) > NameLimit {
		return invalid("Name too long", fmt.Sprintf("A %s can be at most %d characters.", what, NameLimit))
	}

	return nil
}




func checkRequiredName(what, s string) error {
	if strings.TrimSpace(s) == "" {
		return invalid("Name required", fmt.Sprintf("A %s needs a name.", what))
	}

	return checkName(what, s)
}


func checkCoord(what string, v int) error {
	if v < -CoordLimit || v > CoordLimit {
		return invalid("Off the map", fmt.Sprintf("The %s is outside the %d pixel limit.", what, CoordLimit))
	}

	return nil
}




func checkColor(what, s string) error {
	if (len(s) != 7 && len(s) != 9) || s[0] != '#' {
		return invalid("Bad colour", fmt.Sprintf("A %s must be written as #RRGGBB or #RRGGBBAA.", what))
	}

	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return invalid("Bad colour", fmt.Sprintf("A %s must be written as #RRGGBB or #RRGGBBAA.", what))
		}
	}

	return nil
}




func checkPoints(what string, points []int, minPoints, maxNumbers int) error {
	if len(points)%2 != 0 {
		return invalid("Bad shape", fmt.Sprintf("A %s needs an even number of coordinates.", what))
	}
	if len(points) < minPoints*2 {
		return invalid("Bad shape", fmt.Sprintf("A %s needs at least %d points.", what, minPoints))
	}
	if len(points) > maxNumbers {
		return invalid("Too much detail", fmt.Sprintf("A %s can hold at most %d coordinates.", what, maxNumbers))
	}

	for _, v := range points {
		if err := checkCoord("point", v); err != nil {
			return err
		}
	}

	return nil
}




func checkHP(hp, maxHP *int) error {
	if maxHP != nil && (*maxHP < 1 || *maxHP > HPLimit) {
		return invalid("Bad hit points", fmt.Sprintf("Maximum hit points must be between 1 and %d.", HPLimit))
	}
	if hp != nil {
		if *hp < 0 || *hp > HPLimit {
			return invalid("Bad hit points", fmt.Sprintf("Hit points must be between 0 and %d.", HPLimit))
		}
		if maxHP == nil {
			return invalid("Bad hit points", "Hit points need a maximum to go with them.")
		}
	}

	return nil
}




func clampHP(p *Pawn) {
	if p.HP == nil || p.MaxHP == nil {
		return
	}

	v := min(max(*p.HP, 0), *p.MaxHP)
	p.HP = &v
}

func checkAC(ac *int) error {
	if ac != nil && (*ac < 0 || *ac > ACLimit) {
		return invalid("Bad armour class", fmt.Sprintf("Armour class must be between 0 and %d.", ACLimit))
	}

	return nil
}



func checkGrid(g Grid) error {
	if g.CellSize < CellSizeMin || g.CellSize > CellSizeMax {
		return invalid("Bad grid", fmt.Sprintf("Cell size must be between %d and %d pixels.", CellSizeMin, CellSizeMax))
	}
	if err := checkCoord("grid offset", g.OffsetX); err != nil {
		return err
	}
	if err := checkCoord("grid offset", g.OffsetY); err != nil {
		return err
	}
	if err := checkColor("grid colour", g.Color); err != nil {
		return err
	}
	if !g.Lines.Valid() {
		return invalid("Bad grid", "That is not a grid line style.")
	}
	if !g.Snap.Valid() {
		return invalid("Bad grid", "That is not a snapping mode.")
	}
	if !g.Diagonals.Valid() {
		return invalid("Bad grid", "That is not a diagonal rule.")
	}
	if g.FeetPerCell < 1 || g.FeetPerCell > FeetPerCellMax {
		return invalid("Bad grid", fmt.Sprintf("Feet per cell must be between 1 and %d.", FeetPerCellMax))
	}

	return nil
}


func checkCondition(c Condition) error {
	if strings.TrimSpace(c.Name) == "" {
		return invalid("Name required", "A condition needs a name.")
	}
	if utf8.RuneCountInString(c.Name) > ConditionNameLimit {
		return invalid("Name too long", fmt.Sprintf("A condition name can be at most %d characters.", ConditionNameLimit))
	}
	if !c.Color.Valid() {
		return invalid("Bad condition", "That is not a condition colour.")
	}
	if !c.Clear.Valid() {
		return invalid("Bad condition", "A condition clears at the start or the end of a turn.")
	}
	if c.Duration != -1 && c.Duration < 1 {
		return invalid("Bad condition", "A condition lasts for at least one turn, or -1 until it is removed.")
	}
	if c.Duration > InitiativeMax*InitiativeMax {
		return invalid("Bad condition", "That duration is longer than any fight.")
	}

	return nil
}


func checkObjectSize(w, h int) error {
	if w < 1 || w > ObjectPixelsMax || h < 1 || h > ObjectPixelsMax {
		return invalid("Bad size", fmt.Sprintf("An object is between 1 and %d pixels on each side.", ObjectPixelsMax))
	}

	return nil
}








func (s *State) strokeBudget(a Actor, adding int) error {
	total, mine := 0, 0
	for _, st := range s.Strokes {
		total += len(st.Points)
		if st.By == a.ID {
			mine += len(st.Points)
		}
	}

	if total+adding > StrokePointsBudget {
		return invalid("Drawing full", "This room holds as much drawing as it can. Erase something first.")
	}
	if !a.GM() && mine+adding > StrokePointsBudget/PlayerStrokeShare {
		return invalid("Drawing full", "You have drawn as much as one player may. Erase something of yours first.")
	}

	return nil
}



func (s *State) fogBudget(adding int) error {
	total := 0
	for _, f := range s.Fog {
		total += len(f.Points)
	}

	if total+adding > FogPointsBudget {
		return invalid("Fog full", "This room holds as much fog as it can. Clear some first.")
	}

	return nil
}



func checkSelection(n int) error {
	if n > SelectionMax {
		return invalid("Too many pawns", fmt.Sprintf("At most %d pawns can be moved at once.", SelectionMax))
	}

	return nil
}
