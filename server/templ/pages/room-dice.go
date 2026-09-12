package pages

import (
	"strconv"

	"tabletopper/internal/room"
)

const RoomDiceLogID = "room-dice-log"

type RoomDiceData struct {
	RoomID string
	Rolls  []RoomDiceRoll
}

func (d RoomDiceData) Path() string      { return "/fragment/room/dice?room=" + d.RoomID }
func (d RoomDiceData) RollPath() string  { return "/rooms/" + d.RoomID + "/dice" }
func (d RoomDiceData) LogSelect() string { return "#" + RoomDiceLogID }
func (d RoomDiceData) ExprLimit() string { return strconv.Itoa(room.DiceExprLimit) }

type RoomDiceRoll struct {
	ID     string
	Who    string
	Label  string
	Expr   string
	Dice   []RoomDie
	Mod    int
	Total  int
	Crit   bool
	Fumble bool
	Secret bool
	Adv    int
}

type RoomDie struct {
	Value int
	Kept  bool
	Sign  int
}

func (r RoomDiceRoll) TotalText() string { return strconv.Itoa(r.Total) }
func (r RoomDiceRoll) ModText() string {
	switch {
	case r.Mod > 0:
		return "+ " + strconv.Itoa(r.Mod)
	case r.Mod < 0:
		return "- " + strconv.Itoa(-r.Mod)
	}
	return ""
}
func (r RoomDiceRoll) AdvLabel() string {
	switch r.Adv {
	case room.AdvHigh:
		return "Advantage"
	case room.AdvLow:
		return "Disadvantage"
	}
	return ""
}
func (d RoomDie) Text() string {
	if d.Sign < 0 {
		return "-" + strconv.Itoa(d.Value)
	}
	return strconv.Itoa(d.Value)
}
func RollerName(gm bool, name string) string {
	if gm {
		return GameMasterName
	}
	if name == "" {
		return "Someone"
	}
	return name
}
