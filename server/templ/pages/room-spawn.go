package pages
import (
	"net/url"
	"strconv"
	"tabletopper/internal/room"
	"github.com/a-h/templ"
)
const (
	RoomSpawnMonsters = "monsters"
	RoomSpawnTokens   = "tokens"
	RoomSpawnNPCs     = "npcs"
)
const roomSpawnResultsID = "spawn-results"
const (
	NPCDefaultHP = 1
	NPCDefaultAC = 10
)
type RoomSpawnData struct {
	RoomID string
	Kind string
	Query string
	Monsters []MonsterSummary
	Tokens   []RoomSpawnToken
	Avatars  []RoomSpawnAvatar
}
type RoomSpawnToken struct {
	ID   string
	Name string
	Image string
	Width  int
	Height int
}
type RoomSpawnAvatar struct {
	RoomID string
	ID     string
	Name   string
	Image  string
}
func (a RoomSpawnAvatar) NPCPath() string {
	return "/fragment/room/spawn-npc?room=" + a.RoomID + "&asset=" + a.ID
}
func (t RoomSpawnToken) WidthText() string { return pixelText(t.Width) }
func (t RoomSpawnToken) HeightText() string { return pixelText(t.Height) }
func pixelText(value int) string {
	if value < 1 {
		return ""
	}
	return strconv.Itoa(value)
}
func (d RoomSpawnData) IsMonsters() bool { return d.Kind == RoomSpawnMonsters }
func (d RoomSpawnData) IsTokens() bool { return d.Kind == RoomSpawnTokens }
func (d RoomSpawnData) IsNPCs() bool { return d.Kind == RoomSpawnNPCs }
func (d RoomSpawnData) Path(kind string) string {
	return "/fragment/room/spawn?room=" + d.RoomID + "&kind=" + kind
}
func (d RoomSpawnData) ListPath() string {
	return "/fragment/room/spawn-list?room=" + d.RoomID + "&kind=" + d.Kind
}
func (d RoomSpawnData) ResultsID() string { return roomSpawnResultsID }
func (d RoomSpawnData) ActionLabel() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Create monster"
	case RoomSpawnNPCs:
		return "Upload avatar"
	default:
		return "Upload token"
	}
}
func (d RoomSpawnData) UploadPath() string {
	if d.IsNPCs() {
		return "/rooms/" + d.RoomID + "/spawn/avatars"
	}
	return "/rooms/" + d.RoomID + "/spawn/tokens"
}
func (d RoomSpawnData) NewMonsterPath() string {
	return "/fragment/room/spawn-monster?room=" + d.RoomID
}
func (d RoomSpawnData) UploadID() string { return "spawn-upload-" + d.Kind }
func (d RoomSpawnData) SearchLabel() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Search the manual"
	case RoomSpawnNPCs:
		return "Search your faces"
	default:
		return "Search your tokens"
	}
}
func (d RoomSpawnData) NoMatch() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return noMatchHeading("monsters", d.Query)
	case RoomSpawnNPCs:
		return noMatchHeading("faces", d.Query)
	default:
		return noMatchHeading("tokens", d.Query)
	}
}
func (d RoomSpawnData) SearchLimit() string { return strconv.Itoa(AssetNameLimit) }
func (d RoomSpawnData) EmptyHeading() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Nothing in your manual yet."
	case RoomSpawnNPCs:
		return "No faces in your library yet."
	default:
		return "No tokens in your library yet."
	}
}
func (d RoomSpawnData) EmptyBlurb() string {
	switch d.Kind {
	case RoomSpawnMonsters:
		return "Write up what you plan to run in the Monster Manual, then come back and place it."
	case RoomSpawnNPCs:
		return "Upload faces to Avatars on the Assets page. A face is any portrait you want a townsfolk or a villain to wear."
	default:
		return "Upload tokens on the Assets page. A token is any picture you want to put on a table."
	}
}
type RoomSpawnNPCData struct {
	RoomID string
	Query string
	Avatar RoomSpawnAvatar
}
func (d RoomSpawnNPCData) BackPath() string {
	return backToWall(d.RoomID, RoomSpawnNPCs, d.Query)
}
func (d RoomSpawnNPCData) HPValue() string { return strconv.Itoa(NPCDefaultHP) }
func (d RoomSpawnNPCData) ACValue() string { return strconv.Itoa(NPCDefaultAC) }
func (d RoomSpawnNPCData) SizeValue() string { return DefaultSize }
func (d RoomSpawnNPCData) NameLimit() string { return strconv.Itoa(room.NameLimit) }
func (d RoomSpawnNPCData) HPMax() string { return strconv.Itoa(room.HPLimit) }
func (d RoomSpawnNPCData) ACMax() string { return strconv.Itoa(room.ACLimit) }
const RoomSpawnMonsterPanel = "spawn-monster"
type RoomSpawnMonsterData struct {
	RoomID string
	Query string
	Errors []string
}
func (d RoomSpawnMonsterData) CreatePath() string {
	return "/rooms/" + d.RoomID + "/spawn/monsters"
}
func (d RoomSpawnMonsterData) BackPath() string {
	return backToWall(d.RoomID, RoomSpawnMonsters, d.Query)
}
func (d RoomSpawnMonsterData) Panel() string { return RoomSpawnMonsterPanel }
func (d RoomSpawnMonsterData) ErrorTarget() string { return "#" + panelErrorsID(RoomSpawnMonsterPanel) }
func (d RoomSpawnMonsterData) HPValue() string { return strconv.Itoa(NPCDefaultHP) }
func (d RoomSpawnMonsterData) ACValue() string { return strconv.Itoa(NPCDefaultAC) }
func (d RoomSpawnMonsterData) SizeValue() string { return DefaultSize }
func (d RoomSpawnMonsterData) NameLimit() string { return strconv.Itoa(MonsterNameLimit) }
func (d RoomSpawnMonsterData) HPMax() string { return strconv.Itoa(MonsterHPLimit) }
func (d RoomSpawnMonsterData) ACMax() string { return strconv.Itoa(room.ACLimit) }
func backToWall(roomID string, kind string, query string) string {
	path := "/fragment/room/spawn?room=" + roomID + "&kind=" + kind
	if query == "" {
		return path
	}
	return path + "&q=" + url.QueryEscape(query)
}
func npcField(name string) templ.Attributes {
	return templ.Attributes{"data-npc-" + name: true}
}
func named(name string) templ.Attributes {
	return templ.Attributes{"name": name}
}
func monsterImageURL(imageID string) string {
	if imageID == "" {
		return ""
	}
	return "/assets/images/" + imageID
}
func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
