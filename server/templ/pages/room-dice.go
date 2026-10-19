package pages

import (
	"encoding/json"
	"strconv"

	"tabletopper/internal/room"
)

const (
	RoomDiceLogID    = "room-dice-log"
	RoomSavedRollsID = "room-saved-rolls"
	SaveRollPanel    = "save-roll"
)

type RoomDiceData struct {
	RoomID string
	Rolls  []RoomDiceRoll
	Saved  []RoomSavedRoll
}

func (d RoomDiceData) Path() string      { return "/fragment/room/dice?room=" + d.RoomID }
func (d RoomDiceData) RollPath() string  { return diceRollPath(d.RoomID) }
func (d RoomDiceData) LogSelect() string { return "#" + RoomDiceLogID }
func (d RoomDiceData) ExprLimit() string { return strconv.Itoa(room.DiceExprLimit) }
func (d RoomDiceData) SaveFormPath() string {
	return "/fragment/room/dice/save?room=" + d.RoomID
}
func diceRollPath(roomID string) string { return "/rooms/" + roomID + "/dice" }

type RoomSaveRollData struct {
	RoomID   string
	Expr     string
	Problems []string
}

func (d RoomSaveRollData) SavePath() string   { return "/rooms/" + d.RoomID + "/saved-rolls" }
func (d RoomSaveRollData) LabelLimit() string { return strconv.Itoa(room.DiceLabelLimit) }

type RoomSavedRoll struct {
	ID    string
	Label string
	Expr  string
}

func (s RoomSavedRoll) RemovePath() string { return "/saved-rolls/" + s.ID }
func (s RoomSavedRoll) Name() string {
	if s.Label != "" {
		return s.Label
	}
	return s.Expr
}
func (s RoomSavedRoll) RemoveLabel() string { return "Remove " + s.Name() }
func (s RoomSavedRoll) RollVals() string {
	vals, _ := json.Marshal(map[string]string{"expr": s.Expr, "label": s.Label})
	return string(vals)
}

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
