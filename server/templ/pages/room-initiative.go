package pages

import (
	"strconv"

	"tabletopper/internal/events"
)






































































































const RoomInitiativeID = "room-initiative"





















const InitiativeTrigger = events.Initiative + "[!this.hasAttribute('data-dragging')] from:window"





const InitiativeLoadTrigger = "load, " + InitiativeTrigger




const (
	
	
	EntrySolo = "solo"

	
	
	EntryGroup = "group"

	
	
	EntryNamed = "named"
)







const (
	SidePlayer  = "player"
	SideMonster = "monster"
	SideNPC     = "npc"
)




const InitiativePipMax = 12






const initiativeBloodVariants = 9



type RoomInitiativeData struct {
	RoomID string

	
	
	
	IsGM bool

	
	
	
	Empty bool

	Entries []RoomInitiativeEntry
}


type RoomInitiativeEntry struct {
	ID   string
	Name string

	
	Kind string

	
	
	
	
	
	
	
	Side string

	
	
	
	Image string
	Band  string

	
	
	
	
	
	
	Blood string

	
	
	
	HP string

	
	
	Active bool
	Mine   bool

	
	
	Hidden bool

	
	
	
	Solo string

	
	
	Pips  []RoomInitiativePip
	Count string

	
	
	
	Conditions []RoomPawnCondition
}





type RoomInitiativePip struct {
	Band string
}




















func (d RoomInitiativeData) ActingConditions() []RoomPawnCondition {
	for _, e := range d.Entries {
		if e.Active {
			return e.Conditions
		}
	}

	return nil
}





func (d RoomInitiativeData) Path() string {
	return "/fragment/room/initiative?room=" + d.RoomID
}

func (d RoomInitiativeData) Trigger() string { return InitiativeTrigger }





func (d RoomInitiativeData) SyncPath() string {
	return "/rooms/" + d.RoomID + "/initiative/sync"
}

















func (d RoomInitiativeData) NextPath() string {
	return "/rooms/" + d.RoomID + "/initiative/next"
}





func (d RoomInitiativeData) ClearPath() string {
	return "/rooms/" + d.RoomID + "/initiative/clear"
}





func (d RoomInitiativeData) OrderPath() string {
	return "/rooms/" + d.RoomID + "/initiative/order"
}


func (d RoomInitiativeData) ActivatePath(e RoomInitiativeEntry) string {
	return "/rooms/" + d.RoomID + "/initiative/" + e.ID + "/activate"
}

func (d RoomInitiativeData) RemovePath(e RoomInitiativeEntry) string {
	return "/rooms/" + d.RoomID + "/initiative/" + e.ID
}







func (d RoomInitiativeData) ActivateLabel(e RoomInitiativeEntry) string {
	return "Give the turn to " + e.Name
}


func (e RoomInitiativeEntry) Grouped() bool { return e.Kind == EntryGroup }













func (d RoomInitiativeData) ShowHP(e RoomInitiativeEntry) bool {
	return e.HP != "" && (d.IsGM || e.Active)
}



func InitiativePips(bands []string) ([]RoomInitiativePip, string) {
	if len(bands) > InitiativePipMax {
		return nil, "x" + strconv.Itoa(len(bands))
	}

	pips := make([]RoomInitiativePip, 0, len(bands))
	for _, b := range bands {
		pips = append(pips, RoomInitiativePip{Band: b})
	}

	return pips, ""
}










func InitiativeBloodVariant(pawnID string) string {
	var h uint32 = 0x811c9dc5
	for i := range len(pawnID) {
		h ^= uint32(pawnID[i])
		h *= 0x01000193
	}

	return strconv.Itoa(int(h%initiativeBloodVariants) + 1)
}










type RoomInitiativeEntryData struct {
	RoomID string
	Errors []string
}




const RoomInitiativeEntryPanel = "initiative-entry"

func (d RoomInitiativeEntryData) SavePath() string {
	return "/rooms/" + d.RoomID + "/initiative"
}





const EntryNameMax = "128"
