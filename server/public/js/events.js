// The names of the DOM events that cross between scripts which cannot import
// each other: the modules in this directory, which the browser loads as they
// are written, and the room bundle built from server/js/room, which imports
// this file by relative path and inlines it.
//
// THE GO COPY IS internal/uievents, AND A TEST HOLDS THE TWO TOGETHER. Every
// constant here has to match its Go twin by name and value, and neither bundle
// may spell one of these out as a literal anywhere else; the test reads this
// file and greps the rest. Before this each name lived in two or three files
// with a comment saying so, and the failure when they drifted was a panel that
// quietly stopped refetching.
// The room's live panels: raised off the socket, listened for by the fragments'
// hx-trigger attributes.
export const ROOM_PLAYERS = "room:players";
export const ROOM_TABLETOP = "room:tabletop";
export const ROOM_INFO = "room:info";
export const ROOM_INITIATIVE = "room:initiative";
export const ROOM_ROLLS = "room:rolls";
export const ROOM_PAWN = "room:pawn";
// The menu bar's two messages to the renderer.
export const ROOM_VIEW = "room:view";
export const ROOM_BLOOD = "room:blood";
// The socket reaching a window's chrome rather than its content.
export const WINDOW_CLOSE = "window:close";
export const WINDOW_RETITLE = "window:retitle";
// The three dialogs and the toast.
export const ALERT = "alert";
export const MODAL_OPEN = "modal:open";
export const MODAL_CLOSE = "modal:close";
export const TOAST = "flash:toast";
// The account settings, raised when the settings dialog saves.
export const THEME_CHANGE = "theme:change";
export const SETTINGS_CHANGE = "settings:change";
// The sessionStorage key an alert is parked under to survive a navigation.
export const PENDING_ALERT = "alert:pending";
