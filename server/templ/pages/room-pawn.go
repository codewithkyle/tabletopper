package pages

import (
	"strconv"
	"strings"
	"tabletopper/internal/events"

	"github.com/a-h/templ"
)










































































const RoomPawnPanel = "pawn"




const RoomPawnRenamePanel = "pawn-rename"















const pawnSaveTrigger = "input delay:400ms, repeater:changed"





const HPEntryMax = "24"








var ConditionNames = []string{
	"Blinded", "Charmed", "Concentrating", "Deafened", "Exhaustion",
	"Frightened", "Grappled", "Incapacitated", "Invisible", "Paralyzed",
	"Petrified", "Poisoned", "Prone", "Restrained", "Stunned",
	"Unconscious", "Blessed", "Hasted", "Slowed", "Raging",
}








var ConditionColors = []string{"red", "orange", "yellow", "green", "blue", "purple", "pink", "white"}









const ObjectPixelsMax = 8_192


func ConditionColorLabel(color string) string {
	if color == "" {
		return ""
	}

	return strings.ToUpper(color[:1]) + color[1:]
}





var ConditionClears = []Option{
	{Label: "End of turn", Value: "end"},
	{Label: "Start of turn", Value: "start"},
}





















func conditionNamesID(pawnID string) string { return "condition-names-" + pawnID }

func conditionNameList(pawnID string) templ.Attributes {
	return templ.Attributes{"list": conditionNamesID(pawnID)}
}



type RoomPawnData struct {
	RoomID string

	
	
	
	CanEdit bool

	
	
	
	IsGM bool

	Pawn RoomPawn

	
	
	
	
	
	
	
	Layers  []RoomPawnLayer
	LayerID string

	
	
	
	Shown bool
}


type RoomPawnRenameData struct {
	RoomID string
	PawnID string
	Name   string
}







func (d RoomPawnRenameData) SavePath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.PawnID + "/name"
}




type RoomPawn struct {
	ID   string
	Name string

	
	
	Image string

	
	
	
	Object bool

	
	
	
	
	
	
	
	
	
	
	Character bool

	
	
	
	
	HP   string
	Band string

	
	
	
	
	HPValue string
	MaxHP   string

	
	
	AC string

	
	
	
	
	
	
	
	
	
	
	
	Size      string
	SizeValue string
	Pixels    string
	Width     string
	Height    string
	Rotation  string

	
	
	Layer string

	Conditions []RoomPawnCondition

	
	
	Hidden bool

	
	
	
	
	
	
	
	MonsterID string
}
















type RoomPawnCondition struct {
	ID           string
	Name         string
	Color        string
	Duration     string
	DurationText string
	Clear        string
}


type RoomPawnLayer struct {
	ID   string
	Name string
}


func (d RoomPawnData) Panel() string { return RoomPawnPanel + "-" + d.Pawn.ID }



func (d RoomPawnData) ElementID() string { return "pawn-" + d.Pawn.ID }



func (d RoomPawnData) FormID() string { return "pawn-form-" + d.Pawn.ID }

func (d RoomPawnData) ConditionsID() string { return "pawn-conditions-" + d.Pawn.ID }

func (d RoomPawnData) NamesID() string { return conditionNamesID(d.Pawn.ID) }




func (d RoomPawnData) Field(name string) string { return name + "-" + d.Pawn.ID }



func (d RoomPawnData) Path() string {
	return "/fragment/room/pawn?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}




func (d RoomPawnData) RenamePath() string {
	return "/fragment/room/pawn/rename?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}






















func (d RoomPawnData) Trigger() string {
	return events.Pawn + "[detail.id === '" + d.Pawn.ID + "' && !" + typingInPanel + "] from:window"
}


















const typingInPanel = "(this.contains(document.activeElement) && " +
	"(document.activeElement.type === 'text' || document.activeElement.type === 'number'))"






func (d RoomPawnData) HPPath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.Pawn.ID + "/hp"
}








func (d RoomPawnData) StatBlockPath() string {
	return "/fragment/room/stat-block?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}

func (d RoomPawnData) StatBlockWindow() string { return "monster:" + d.Pawn.MonsterID }




func (d RoomPawnData) SavePath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.Pawn.ID
}





func (d RoomPawnData) RemovePath() string {
	return "/rooms/" + d.RoomID + "/pawns"
}












func (d RoomPawnData) RemoveLabel() string {
	return "Remove " + d.Pawn.Name + " from the table"
}



func (d RoomPawnData) CanRename() bool {
	return d.CanEdit && !d.Pawn.Character
}








func (d RoomPawnData) HasActions() bool {
	return d.IsGM || d.CanRename()
}











func (d RoomPawnData) StatBlockLabel() string { return "Stat block for " + d.Pawn.Name }

func (d RoomPawnData) RenameLabel() string { return "Rename " + d.Pawn.Name }

func (d RoomPawnData) VisibleLabel() string { return "Players can see " + d.Pawn.Name }



func (d RoomPawnData) RemovePrompt() string {
	return "Remove " + d.Pawn.Name + " from the table. This cannot be undone."
}





func (d RoomPawnData) RemoveVals() string {
	return `{"ids": "` + d.Pawn.ID + `"}`
}
















func (d RoomPawnData) AddTurnPath() string {
	return "/rooms/" + d.RoomID + "/initiative"
}

func (d RoomPawnData) AddTurnVals() string {
	return `{"pawn": "` + d.Pawn.ID + `"}`
}

func (d RoomPawnData) AddTurnLabel() string {
	return "Add " + d.Pawn.Name + " to the initiative tracker"
}







func (d RoomPawnData) ConditionRowPath() string {
	return "/fragment/room/condition-row?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}



func PawnHPText(hp *int, maxHP *int) string {
	if hp == nil {
		return ""
	}
	if maxHP == nil {
		return strconv.Itoa(*hp)
	}

	return strconv.Itoa(*hp) + " / " + strconv.Itoa(*maxHP)
}








func PawnBandText(band string) string {
	switch band {
	case "healthy":
		return "Healthy"
	case "bruised":
		return "Bruised"
	case "bloody":
		return "Bloody"
	case "veryBloody":
		return "Very bloody"
	case "nearDeath":
		return "Near death"
	case "dead":
		return "Dead"
	}

	return ""
}


func PawnSizeText(size string) string {
	if size == "" {
		return ""
	}

	return strings.ToUpper(size[:1]) + size[1:]
}









func PawnPixelsText(w int, h int, rotation int) string {
	if w <= 0 || h <= 0 {
		return ""
	}

	out := strconv.Itoa(w) + " by " + strconv.Itoa(h) + " pixels"
	if rotation != 0 {
		out += ", turned " + strconv.Itoa(rotation) + " degrees"
	}

	return out
}




func PawnDurationText(turns int) string {
	if turns < 0 {
		return ""
	}

	
	
	
	if turns == 1 {
		return "1 turn left"
	}

	return strconv.Itoa(turns) + " turns left"
}



func PawnDurationValue(turns int) string { return strconv.Itoa(turns) }
