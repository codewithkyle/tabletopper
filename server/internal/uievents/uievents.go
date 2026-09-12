package uievents

const (
	Players    = "room:players"
	Tabletop   = "room:tabletop"
	Info       = "room:info"
	Initiative = "room:initiative"
	Rolls      = "room:rolls"
	Music      = "room:music"
	Pawn       = "room:pawn"
	Character  = "room:character"
)
const (
	View  = "room:view"
	Blood = "room:blood"
)
const (
	WindowClose   = "window:close"
	WindowRetitle = "window:retitle"
)
const (
	Alert      = "alert"
	ModalOpen  = "modal:open"
	ModalClose = "modal:close"
	Toast      = "flash:toast"
)
const (
	ThemeChange    = "theme:change"
	SettingsChange = "settings:change"
)
const PendingAlert = "alert:pending"

var All = map[string]string{
	"ROOM_PLAYERS":    Players,
	"ROOM_TABLETOP":   Tabletop,
	"ROOM_INFO":       Info,
	"ROOM_INITIATIVE": Initiative,
	"ROOM_ROLLS":      Rolls,
	"ROOM_MUSIC":      Music,
	"ROOM_PAWN":       Pawn,
	"ROOM_CHARACTER":  Character,
	"ROOM_VIEW":       View,
	"ROOM_BLOOD":      Blood,
	"WINDOW_CLOSE":    WindowClose,
	"WINDOW_RETITLE":  WindowRetitle,
	"ALERT":           Alert,
	"MODAL_OPEN":      ModalOpen,
	"MODAL_CLOSE":     ModalClose,
	"TOAST":           Toast,
	"THEME_CHANGE":    ThemeChange,
	"SETTINGS_CHANGE": SettingsChange,
	"PENDING_ALERT":   PendingAlert,
}
