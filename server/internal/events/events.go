// Package events is the names of the DOM events that cross between the
// server's HX-Trigger headers, the templ pages' hx-trigger attributes, and the
// two browser bundles -- which cannot import each other.
//
// ONE SOURCE, PINNED TO THE OTHER. server/public/js/events.js exports the same
// names for the browser, and TestTheBrowserAgreesOnEveryEventName reads that
// file and fails when the two disagree, or when either bundle spells one of
// these out as a literal anywhere else. Before this each name was a string
// written in two or three files with a comment saying so, and the failure when
// they drifted was silent: a panel that stopped refetching.
//
// WHY THEY ARE HERE AND NOT IN templ/pages: internal/htmx raises three of them
// and does not import the pages, and the pages do not import htmx. A package
// with no dependencies is the one place both can reach.
package events

// The room's live panels. panels.ts raises these off the socket and the
// fragments' hx-trigger attributes listen on window.
//
// THE NAMES ARE CHOSEN AGAINST TAILWIND AS WELL AS AGAINST EACH OTHER. An
// attribute value in a .templ file is scanned for class candidates and split on
// the colon, so "room:table" would have put DaisyUI's whole table family into
// the stylesheet; see the note on .templ files in CLAUDE.md. The half after the
// colon must not be a component name.
const (
	Players    = "room:players"
	Tabletop   = "room:tabletop"
	Info       = "room:info"
	Initiative = "room:initiative"
	Pawn       = "room:pawn"
)

// The menu bar's two messages to the renderer, across the bundle boundary.
const (
	View  = "room:view"
	Blood = "room:blood"
)

// The socket reaching a window's chrome rather than its content.
const (
	WindowClose   = "window:close"
	WindowRetitle = "window:retitle"
)

// The three dialogs and the toast, raised by internal/htmx and by markup.
const (
	Alert      = "alert"
	ModalOpen  = "modal:open"
	ModalClose = "modal:close"
	Toast      = "flash:toast"
)

// The account settings, raised by internal/htmx when the settings dialog
// saves, and read by whatever is on screen at the time.
const (
	ThemeChange    = "theme:change"
	SettingsChange = "settings:change"
)

// PendingAlert is not an event but the same kind of contract: the
// sessionStorage key under which the room bundle parks an alert for the next
// page to show, because a dialog opened a moment before a navigation is a
// dialog nobody reads.
const PendingAlert = "alert:pending"

// All is every name above, keyed by the identifier the browser module exports
// it under. It is what the test walks, and adding a constant without a line
// here is what that test catches first.
var All = map[string]string{
	"ROOM_PLAYERS":    Players,
	"ROOM_TABLETOP":   Tabletop,
	"ROOM_INFO":       Info,
	"ROOM_INITIATIVE": Initiative,
	"ROOM_PAWN":       Pawn,
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
