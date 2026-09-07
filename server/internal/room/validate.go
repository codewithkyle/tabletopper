package room

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// THE LIMITS. Every one of them is enforced on the server, which is the only
// place enforcement means anything: the client's own checks are a courtesy that
// stops a person typing something silly, and a socket is open to whatever
// somebody wants to send down it.
//
// THEY ARE ALL NAMED, and the names appear in the error messages, because the
// question a GM asks when a limit stops them is "what is the number" and the
// answer should not require reading the source. They are generous on purpose:
// each one is set where a real table would never reach it and a script would
// immediately, so the honest use is never the thing that trips.
const (
	// NameLimit is 128 runes, matching the room name column and every other
	// name in this app. Runes rather than bytes: a name in Japanese is not a
	// third of a name.
	NameLimit = 128

	// ConditionNameLimit is shorter because a condition is drawn as a chip
	// under a pawn, and a 128-character chip is a wall.
	ConditionNameLimit = 64

	// CoordLimit bounds every coordinate at a million map pixels in either
	// direction. The largest map anyone will tile is tens of thousands across,
	// so this is not a constraint on maps -- it is a bound on the arithmetic,
	// so that a group move cannot be handed a delta that overflows into a
	// position no renderer can express.
	CoordLimit = 1_000_000

	// HPLimit and ACLimit are 5e's numbers with room to spare. A tarrasque has
	// 676 hit points and the highest printed armour class is in the twenties.
	HPLimit = 9_999
	ACLimit = 99

	// CellSizeMin and CellSizeMax bound the grid. Below eight pixels a cell is
	// smaller than the line drawn around it; above five hundred a screen holds
	// four of them.
	CellSizeMin = 8
	CellSizeMax = 512

	// StrokeWidthMax is a brush, not a fill tool.
	StrokeWidthMax = 64

	// StrokeChunkMax bounds one begin or extend, which is what keeps a frame
	// well under the socket's cap while a fast stylus is drawing. Points go out
	// at roughly ten hertz, so this is about a hundred samples per chunk more
	// than any hand produces.
	StrokeChunkMax = 512

	// StrokePointsMax bounds one whole stroke, so a client that never sends
	// stroke.end cannot grow one line without bound.
	StrokePointsMax = 20_000

	// StrokesMax and FogShapesMax bound the two collections a session
	// accumulates rather than replaces. Five thousand strokes is a session
	// nobody has had; the number exists so that the snapshot column has a
	// ceiling.
	StrokesMax   = 5_000
	FogShapesMax = 2_000

	// FogPointsMax bounds one polygon. A hand-drawn fog outline is tens of
	// points and this allows a thousand of them.
	FogPointsMax = 2_000

	// PawnsMax is the table's population. The renderer's stress target is five
	// hundred and this is twice it.
	PawnsMax = 1_000

	// ConditionsMax is per pawn. Sixteen chips around one disc is already
	// unreadable, which is the real limit; this is the one that can be checked.
	ConditionsMax = 16

	// SelectionMax bounds one move, drag or remove. Two hundred pawns dragged
	// as a group is a marquee across the whole table, and beyond it the
	// per-command cost stops being flat.
	SelectionMax = 200

	// FootprintMax is an object's size on one axis, in cells. Twenty cells is a
	// hundred feet of ship.
	FootprintMax = 20

	// InitiativeMax is entries in the tracker. A round with two hundred turns
	// in it is not a round.
	InitiativeMax = 200

	// LayersMax is floors per room. Twenty is a tower.
	LayersMax = 20
)

// The grid's defaults, named because NewState and the validation of a reset
// both need them and a literal 64 in two places is a literal 64 that drifts.
const (
	DefaultCellSize    = 64
	DefaultFeetPerCell = 5

	// DefaultGridColor is opaque black. The alpha is written out rather than
	// left to a six-digit shorthand so that a GM opening the colour picker sees
	// the channel they are most likely to want to change.
	DefaultGridColor = "#000000FF"
)

// FeetPerCellMax bounds the distance scale. Five feet is a 5e cell and a
// hundred is a hex on an overland map; past that the ruler stops being a ruler.
const FeetPerCellMax = 1_000

// checkName caps a name that is allowed to be empty -- a pawn dropped from a
// token image has a picture instead of a name, and that is a legitimate pawn.
func checkName(what, s string) error {
	if utf8.RuneCountInString(s) > NameLimit {
		return invalid("Name too long", fmt.Sprintf("A %s can be at most %d characters.", what, NameLimit))
	}

	return nil
}

// checkRequiredName is for the names that are the whole of what the reader
// sees: a layer in the layer manager, a line in the initiative tracker. Empty
// is refused after trimming, so a name of three spaces is refused too.
func checkRequiredName(what, s string) error {
	if strings.TrimSpace(s) == "" {
		return invalid("Name required", fmt.Sprintf("A %s needs a name.", what))
	}

	return checkName(what, s)
}

// checkCoord bounds one coordinate.
func checkCoord(what string, v int) error {
	if v < -CoordLimit || v > CoordLimit {
		return invalid("Off the map", fmt.Sprintf("The %s is outside the %d pixel limit.", what, CoordLimit))
	}

	return nil
}

// checkColor accepts #RRGGBB and #RRGGBBAA and nothing else. Named CSS colours
// and rgb() are refused because the renderer parses this into four floats and
// a shader does not have a colour table.
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

// checkPoints validates a flat x,y array. minPoints is pairs, not numbers,
// because that is how the shapes are described: a rectangle is two corners and
// a polygon is three vertices.
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

// checkHP validates the pair together, because they only mean anything
// together: hit points without a maximum have no band and no bar to draw, and
// the clamp below is the reason a pawn can never be at 12 of 10.
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

// clampHP is the other half of the pair. Damage arrives as a new value rather
// than as a delta, and a client that subtracts past zero is showing a number
// the server will not store.
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

// checkGrid validates every field of the grid at once, because setGrid replaces
// the whole object and a half-applied grid is a table nobody can line up.
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

// checkCondition validates one status chip.
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

// checkFootprint validates an object's rectangle.
func checkFootprint(w, h int) error {
	if w < 1 || w > FootprintMax || h < 1 || h > FootprintMax {
		return invalid("Bad footprint", fmt.Sprintf("An object is between 1 and %d cells on each side.", FootprintMax))
	}

	return nil
}

// checkSelection bounds the ids in one move, drag or remove. The anchor counts,
// which is why the callers pass len(others)+1.
func checkSelection(n int) error {
	if n > SelectionMax {
		return invalid("Too many pawns", fmt.Sprintf("At most %d pawns can be moved at once.", SelectionMax))
	}

	return nil
}
