package pages

import (
	"strconv"

	"tabletopper/internal/room"
)

const (
	ScenesWindow   = "scenes"
	SceneSavePanel = "room-scene-save"
	SceneNameLimit = room.NameLimit
)

type SceneCard struct {
	RoomID    string
	ID        string
	Name      string
	PreviewID string
	Updated   Timestamp
	Open      bool
	Autosave  bool
}
type RoomScenesData struct {
	RoomID string
	Scenes []SceneCard
}
type RoomSceneSaveData struct {
	RoomID string
}

func ScenesWindowPath(roomID string) string {
	return "/fragment/room/scenes?room=" + roomID
}
func (d RoomScenesData) Path() string {
	return ScenesWindowPath(d.RoomID)
}
func (d RoomScenesData) SaveFormPath() string {
	return "/fragment/room/scene/save?room=" + d.RoomID
}
func (d RoomScenesData) EmptyHeading() string { return roomScenesEmptyHeading }
func (d RoomScenesData) EmptyBlurb() string   { return roomScenesEmptyBlurb }
func (d RoomSceneSaveData) SavePath() string {
	return "/rooms/" + d.RoomID + "/scenes"
}
func (d RoomSceneSaveData) NameLimit() string {
	return strconv.Itoa(SceneNameLimit)
}
func (d RoomSceneSaveData) Blurb() string {
	return sceneSaveBlurb
}
func (c SceneCard) PreviewURL() string {
	return "/assets/images/" + c.PreviewID + "/preview"
}
func (c SceneCard) HasPreview() bool {
	return c.PreviewID != ""
}
func (c SceneCard) OpenPath() string {
	return "/rooms/" + c.RoomID + "/scenes/" + c.ID + "/open"
}
func (c SceneCard) OpenPrompt() string {
	return sceneOpenPrompt
}
func (c SceneCard) OpenHeading() string {
	return "Open " + c.Name + "?"
}
func (c SceneCard) NamePath() string {
	return "/scenes/" + c.ID + "/name"
}
func (c SceneCard) DuplicatePath() string {
	return "/scenes/" + c.ID + "/duplicate"
}
func (c SceneCard) DeletePath() string {
	return "/scenes/" + c.ID
}
func (c SceneCard) DeletePrompt() string {
	return "Delete " + c.Name + ". This cannot be undone, and a room with it open is left exactly as it is."
}
func (c SceneCard) NameLimit() string {
	return strconv.Itoa(SceneNameLimit)
}
func (c SceneCard) AutosavePath() string {
	return "/scenes/" + c.ID + "/autosave"
}
func (c SceneCard) SavePath() string {
	return "/rooms/" + c.RoomID + "/scenes/" + c.ID + "/save"
}
func (c SceneCard) SavePrompt() string {
	return sceneOverwritePrompt
}
func (c SceneCard) SaveHeading() string {
	return "Overwrite " + c.Name + "?"
}
func (c SceneCard) AutosaveHint() string {
	return sceneAutosaveHint
}
func (c SceneCard) AutosaveLabel() string {
	return sceneAutosaveLabel
}
func (c SceneCard) CardClass() string {
	if c.Open {
		return sceneOpenBox
	}
	return sceneCardBox
}

const (
	roomScenesEmptyHeading = "No scenes yet."
	roomScenesEmptyBlurb   = "Lay the tabletop out the way you want it and save it here. Opening it again puts the maps, the fog, the drawing and every monster back where they were, for everybody at the table."
	sceneAutosaveLabel     = "Autosave changes"
	sceneAutosaveHint      = "Autosave changes: what happens on this scene is written back to it when another scene is opened, when the tabletop is cleared, when the last GM disconnects, and when the room is closed or the server restarts. Leave it off for an encounter you run more than once."
	sceneOverwritePrompt   = "This scene is replaced by what is on the tabletop now. The version you saved before is gone."
	sceneOpenPrompt        = "Every pawn on the tabletop goes, the party's included, along with the fog, the drawing and the turn order. This cannot be undone."
	sceneSaveBlurb         = "The grid, the floors and their maps, the fog, the drawing and every pawn that is not a person's. Nothing about the people at this table is saved with it."
)
