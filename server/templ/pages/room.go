package pages

import (
	"slices"
	"strconv"
	"strings"

	"tabletopper/internal/prefs"
	"tabletopper/internal/room"
)

























































const tooltipBody = "tooltip-content"





const roomLockID = "room-lock"



















const (
	DefaultRoomTool = "select"
	RoomToolMove    = "move"
	RoomToolMeasure = "measure"
	RoomToolFog     = "fog"
	RoomToolDraw    = "draw"
	RoomToolPing    = "ping"
)





type RoomPageData struct {
	ID   string
	Name string

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	Code string

	Locked bool
	Closed bool

	
	
	
	Role room.Role

	
	
	
	
	
	
	
	
	
	
	UserID string

	
	
	
	
	
	Socket string

	
	
	
	Version string

	
	
	
	
	Debug bool

	
	
	
	
	
	
	
	
	
	
	
	
	
	FollowTurn bool

	
	
	
	
	
	
	
	
	
	ShowBlood bool

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	PingVolume int
}


func (d RoomPageData) PingVolumeAttr() string {
	return strconv.Itoa(prefs.ClampPingVolume(d.PingVolume))
}





func (d RoomPageData) Bundle() string {
	if d.Version == "" {
		return "/static/room.js"
	}

	return "/static/room.js?v=" + d.Version
}





func (d RoomPageData) MembersPath() string {
	return "/fragment/room/members?room=" + d.ID
}




func (d RoomPageData) SpawnPath() string {
	return "/fragment/room/spawn?room=" + d.ID + "&kind=" + RoomSpawnMonsters
}




func (d RoomPageData) ClearPath() string {
	return "/rooms/" + d.ID + "/tabletop/clear"
}





func (d RoomPageData) PartyPath() string {
	return "/rooms/" + d.ID + "/pawns/party"
}





func (d RoomPageData) LayersPath() string {
	return "/fragment/room/layers?room=" + d.ID
}

func (d RoomPageData) GridPath() string {
	return "/fragment/room/grid?room=" + d.ID
}




func (d RoomPageData) FogFillPath() string {
	return "/rooms/" + d.ID + "/fog/fill"
}

func (d RoomPageData) FogClearPath() string {
	return "/rooms/" + d.ID + "/fog/clear"
}

func (d RoomPageData) DrawingClearPath() string {
	return "/rooms/" + d.ID + "/drawing/clear"
}

func (d RoomPageData) LayerNamePath() string {
	return "/fragment/room/layer?room=" + d.ID
}







func (d RoomPageData) InitiativePath() string {
	return "/fragment/room/initiative?room=" + d.ID
}


func (d RoomPageData) InitiativeEntryPath() string {
	return "/fragment/room/initiative/entry?room=" + d.ID
}




func (d RoomPageData) InitiativeSyncPath() string {
	return "/rooms/" + d.ID + "/initiative/sync"
}

func (d RoomPageData) InitiativeNextPath() string {
	return "/rooms/" + d.ID + "/initiative/next"
}

func (d RoomPageData) InitiativeClearPath() string {
	return "/rooms/" + d.ID + "/initiative/clear"
}



func (d RoomPageData) IsGM() bool {
	return d.Role == room.RoleGM
}





func (d RoomPageData) RoleName() string {
	return string(d.Role)
}


type RoomMenu struct {
	Label string
	Items []RoomMenuItem
}

















type RoomMenuItem struct {
	Label string
	ID    string

	Href   string
	NewTab bool

	Post           string
	Confirm        string
	ConfirmHeading string
	ConfirmLabel   string

	Action string
	Value  string

	
	
	
	
	
	
	
	
	Key string

	
	
	Window RoomWindow

	
	
	
	
	Modal RoomModal

	
	
	Danger bool

	
	
	
	
	
	
	
	
	
	
	
	Layered bool

	Disabled bool
}
























func (d RoomPageData) Menus() []RoomMenu {
	menus := []RoomMenu{d.roomMenu(), d.tabletopMenu()}

	if d.IsGM() {
		menus = append(menus, d.fogMenu(), d.initiativeMenu())
	} else {
		menus = append(menus, characterMenu())
	}

	return append(menus,
		d.toolsMenu(),
		d.viewMenu(),
		helpMenu(),
	)
}




















func (d RoomPageData) toolsMenu() RoomMenu {
	if !d.IsGM() {
		return RoomMenu{Label: "Tools", Items: comingSoon("Dice tray")}
	}

	return RoomMenu{Label: "Tools", Items: comingSoon("Monster Manual", "Dice tray")}
}










func characterMenu() RoomMenu {
	return RoomMenu{Label: "Character", Items: comingSoon("Character sheet", "Journal")}
}


















func (d RoomPageData) roomMenu() RoomMenu {
	items := []RoomMenuItem{}

	if d.IsGM() && !d.Closed {
		items = append(items, roomLockItem(d))
	}
	if d.IsGM() && d.Closed {
		items = append(items, RoomMenuItem{Label: "Reopen room", Post: "/rooms/" + d.ID + "/open"})
	}

	items = append(items, RoomMenuItem{Label: "Player List", Window: RoomWindow{
		ID:     "players",
		Title:  "Players",
		URL:    d.MembersPath(),
		Width:  260,
		Height: 260,
	}})

	if !d.Closed {
		items = append(items, RoomMenuItem{Label: "Copy room code", Action: "copy-code", Value: d.Code})
	}

	if d.IsGM() {
		items = append(items, RoomMenuItem{Label: "Back to rooms", Href: "/rooms"})
		if !d.Closed {
			items = append(items, RoomMenuItem{
				Label:          "Close room",
				Post:           "/rooms/" + d.ID + "/close",
				Confirm:        "Everyone in this room will be removed and the code will stop working.",
				ConfirmHeading: "Close this room?",
				ConfirmLabel:   "Close room",
				Danger:         true,
			})
		}

		return RoomMenu{Label: "Room", Items: items}
	}

	return RoomMenu{Label: "Room", Items: append(items, RoomMenuItem{
		Label:  "Leave room",
		Post:   "/rooms/" + d.ID + "/leave",
		Danger: true,
	})}
}





func roomLockItem(d RoomPageData) RoomMenuItem {
	if d.Locked {
		return RoomMenuItem{ID: roomLockID, Label: "Unlock room", Post: "/rooms/" + d.ID + "/unlock"}
	}

	return RoomMenuItem{ID: roomLockID, Label: "Lock room", Post: "/rooms/" + d.ID + "/lock"}
}









































func (d RoomPageData) tabletopMenu() RoomMenu {
	blood := RoomMenuItem{Label: "Clear blood", Action: roomBloodAction}

	if !d.IsGM() {
		return RoomMenu{Label: "Tabletop", Items: []RoomMenuItem{blood}}
	}

	return RoomMenu{Label: "Tabletop", Items: []RoomMenuItem{
		{Label: "Layers", Window: RoomWindow{
			ID:     "layers",
			Title:  "Layers",
			URL:    d.LayersPath(),
			Width:  320,
			Height: 360,
		}},
		{Label: "Grid & settings", Window: RoomWindow{
			ID:     "grid",
			Title:  "Grid & settings",
			URL:    d.GridPath(),
			Width:  300,
			Height: 420,
		}},
		{Label: "Spawn pawns", Post: d.PartyPath()},
		{Label: "Spawn from library", Modal: RoomModal{URL: d.SpawnPath(), Size: "lg"}},
		blood,
		{
			Label:          "Clear drawing",
			Post:           d.DrawingClearPath(),
			Layered:        true,
			Confirm:        "Every line drawn on the floor you are looking at goes, for everybody at the table. This cannot be undone.",
			ConfirmHeading: "Clear this floor's drawing?",
			ConfirmLabel:   "Clear drawing",
		},
		{
			Label:          "Clear tabletop",
			Post:           d.ClearPath(),
			Confirm:        "Every map, pawn, fog shape and drawing goes, on every floor, and the initiative tracker is emptied. The floors themselves stay, and so does the grid.",
			ConfirmHeading: "Clear the tabletop?",
			ConfirmLabel:   "Clear tabletop",
			Danger:         true,
		},
	}}
}



















func (d RoomPageData) fogMenu() RoomMenu {
	return RoomMenu{Label: "Fog", Items: []RoomMenuItem{
		{
			Label:          "Fill fog",
			Post:           d.FogFillPath(),
			Layered:        true,
			Confirm:        "The floor you are looking at is covered, and every area you have uncovered on it is forgotten. This cannot be undone.",
			ConfirmHeading: "Cover this floor?",
			ConfirmLabel:   "Fill fog",
		},
		{
			Label:          "Clear fog",
			Post:           d.FogClearPath(),
			Layered:        true,
			Confirm:        "The floor you are looking at is uncovered for everybody, and every area you have uncovered on it is forgotten. This cannot be undone.",
			ConfirmHeading: "Uncover this floor?",
			ConfirmLabel:   "Clear fog",
			Danger:         true,
		},
	}}
}




























func (d RoomPageData) initiativeMenu() RoomMenu {
	return RoomMenu{Label: "Initiative", Items: []RoomMenuItem{
		{Label: "Sync tracker", Post: d.InitiativeSyncPath()},
		{Label: "Add entry", Modal: RoomModal{URL: d.InitiativeEntryPath(), Size: "sm"}},
		{Label: "Next turn", Post: d.InitiativeNextPath(), Key: "N"},
		{
			Label:          "Clear tracker",
			Post:           d.InitiativeClearPath(),
			Confirm:        "The whole turn order goes, on every screen. Nothing on the tabletop is touched, and Sync tracker builds it again.",
			ConfirmHeading: "Clear the initiative tracker?",
			ConfirmLabel:   "Clear tracker",
			Danger:         true,
		},
	}}
}













func (d RoomPageData) viewMenu() RoomMenu {
	return RoomMenu{Label: "View", Items: []RoomMenuItem{
		{Label: "Zoom in", Action: roomViewAction, Value: "zoom-in"},
		{Label: "Zoom out", Action: roomViewAction, Value: "zoom-out"},
		{Label: "100%", Action: roomViewAction, Value: "zoom-1"},
		{Label: "200%", Action: roomViewAction, Value: "zoom-2"},
		{Label: "Fit map", Action: roomViewAction, Value: "fit"},
		{Label: "Toggle fullscreen", Action: "fullscreen"},
	}}
}










const (
	roomViewAction  = "view"
	roomBloodAction = "clear-blood"
)
























func helpMenu() RoomMenu {
	return RoomMenu{Label: "Help", Items: []RoomMenuItem{
		{Label: "Settings", Modal: RoomModal{URL: AccountSettingsPath}},
		{Label: "Report issue", Disabled: true},
		{Label: "Privacy policy", Href: "/privacy", NewTab: true},
		{Label: "Terms of service", Href: "/tos", NewTab: true},
	}}
}






func comingSoon(labels ...string) []RoomMenuItem {
	items := make([]RoomMenuItem, 0, len(labels))
	for _, label := range labels {
		items = append(items, RoomMenuItem{Label: label, Disabled: true})
	}

	return items
}





























type RoomTool struct {
	Name  string
	Label string

	
	
	Pans bool

	
	
	
	Measures bool

	
	
	
	Fogs bool

	
	
	
	
	
	
	
	
	
	
	Draws bool

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	Pings bool

	
	
	
	
	GM bool

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	Key string
}








func RoomTools() []RoomTool {
	return []RoomTool{
		{Name: DefaultRoomTool, Label: "Select", Key: "v"},
		{Name: RoomToolMove, Label: "Move", Pans: true, Key: "h"},
		{Name: RoomToolMeasure, Label: "Measure", Measures: true, Key: "m"},
		{Name: RoomToolFog, Label: "Fog", Fogs: true, GM: true, Key: "f"},
		{Name: RoomToolDraw, Label: "Draw", Draws: true, Key: "d"},
		{Name: RoomToolPing, Label: "Ping", Pings: true, Key: "p"},
	}
}



func (d RoomPageData) Tools() []RoomTool {
	all := RoomTools()
	if d.IsGM() {
		return all
	}

	mine := make([]RoomTool, 0, len(all))
	for _, t := range all {
		if !t.GM {
			mine = append(mine, t)
		}
	}

	return mine
}




























const (
	DrawColorPanelID = "draw-color-panel"
	DrawWidthPanelID = "draw-width-panel"

	
	
	
	DrawWidthDefault = "4"
)


var DrawWidthMax = strconv.Itoa(room.StrokeWidthMax)





































func DrawModeChoices() []Choice {
	return []Choice{
		{Value: "pen", Label: "Pen", Hint: "Drag to draw."},
		{Value: "rect", Label: "Rectangle", Hint: "Drag a box. It measures both sides as you go."},
		{Value: "circle", Label: "Circle", Hint: "Drag out from the centre. It measures the radius as you go."},
		{Value: "cone", Label: "Cone", Hint: "Drag from the point. It is as wide as it is long."},
		{Value: "erase", Label: "Eraser", Hint: "Drag over a line to rub it out. Ctrl+Z takes back your last one."},
	}
}













func FogShapeChoices() []Choice {
	return []Choice{
		{Value: "rect", Label: "Rectangle", Hint: "Drag a box."},
		{Value: "poly", Label: "Polygon", Hint: "Click each corner. Right click closes it."},
	}
}

func FogModeChoices() []Choice {
	return []Choice{
		{Value: "reveal", Label: "Uncover", Hint: "Cut a hole in the fog."},
		{Value: "hide", Label: "Cover", Hint: "Put the fog back."},
	}
}




func (t RoomTool) Pressed() string {
	return strconv.FormatBool(t.Name == DefaultRoomTool)
}




func (t RoomTool) KeyLabel() string {
	return strings.ToUpper(t.Key)
}





const emptyVals = "{}"











































type RoomWindow struct {
	
	
	
	ID    string
	Title string
	URL   string

	
	
	Width  int
	Height int
}





func (w RoomWindow) WidthValue() string { return dimension(w.Width) }

func (w RoomWindow) HeightValue() string { return dimension(w.Height) }

func dimension(value int) string {
	if value <= 0 {
		return ""
	}

	return strconv.Itoa(value)
}




type RoomModal struct {
	URL string

	
	Size string
}

















type RoomMember struct {
	
	
	
	
	ID string

	
	
	Name string

	
	
	Username string

	Avatar string

	
	
	IsGM bool

	
	
	
	Connected bool
}

















type RoomMembersData struct {
	RoomID  string
	Members []RoomMember

	
	
	
	
	
	
	
	CanKick bool

	
	
	
	
	
	Live bool
}



func (d RoomMembersData) Path() string {
	return "/fragment/room/members?room=" + d.RoomID
}





const GameMasterName = "Game Master"









func MemberName(isGM bool, character string, username string) string {
	if isGM {
		return GameMasterName
	}
	if character != "" {
		return character
	}

	return username
}






func (m RoomMember) ShowUsername() bool {
	return m.Name != m.Username
}





func (d RoomMembersData) KickPath(m RoomMember) string {
	return "/rooms/" + d.RoomID + "/players/" + m.ID + "/kick"
}




func (d RoomMembersData) KickPrompt(m RoomMember) string {
	return "Remove " + m.Name + " from the room? Their pawns stay on the table, and they can join again with the code unless you lock the room."
}




func SortRoomMembers(members []RoomMember) []RoomMember {
	slices.SortFunc(members, func(a, b RoomMember) int {
		if a.IsGM != b.IsGM {
			if a.IsGM {
				return -1
			}

			return 1
		}

		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return members
}
