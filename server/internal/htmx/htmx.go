// Package htmx writes the response headers htmx reads: HX-Trigger for the
// alert and toast events the page listens for, HX-Redirect and HX-Refresh
// for navigation. A handler answering an htmx request uses these instead of
// a 3xx, which fetch() would follow and swap into the page.
package htmx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"tabletopper/internal/events"
)

// trigger adds events to the response's HX-Trigger header, merging with any
// already there so a Toast followed by an Error keeps both. The header is
// JSON, and it is built as JSON: a character named `Say "hi"` used to break
// it and lose the toast.
func trigger(w http.ResponseWriter, events map[string]any) {
	merged := map[string]any{}
	if existing := w.Header().Get("HX-Trigger"); existing != "" {
		if err := json.Unmarshal([]byte(existing), &merged); err != nil {
			slog.Error("Existing HX-Trigger header is not JSON; replacing it", "header", existing)
			merged = map[string]any{}
		}
	}
	for name, detail := range events {
		merged[name] = detail
	}

	b, err := json.Marshal(merged)
	if err != nil {
		slog.Error("Failed to encode HX-Trigger", "error", err)
		return
	}
	w.Header().Set("HX-Trigger", string(b))
}

// Error raises an alert with the given heading and message under status. The
// body stays empty; the noSwap config in base.templ leaves the caller's
// target alone for every 4xx and 5xx.
func Error(w http.ResponseWriter, heading string, msg string, status int) {
	trigger(w, map[string]any{
		events.Alert: map[string]string{"heading": heading, "message": msg},
	})
	w.WriteHeader(status)
}

// ServerError is the generic 500.
func ServerError(w http.ResponseWriter) {
	Error(w, "Server Error", "Something went wrong on the server. If this continues to happen submit an issue on GitHub.", http.StatusInternalServerError)
}

// NotFound is the 404 for a resource the page still shows but the caller no
// longer owns or that no longer exists. what names it: "character", "map".
func NotFound(w http.ResponseWriter, what string) {
	Error(w, "Not Found", "That "+what+" no longer exists. Refresh the page and try again.", http.StatusNotFound)
}

// Redirect asks htmx to navigate to path. The status is 200 so htmx reads
// the header rather than treating the response as an error.
func Redirect(w http.ResponseWriter, path string) {
	w.Header().Set("HX-Redirect", path)
	w.WriteHeader(http.StatusOK)
}

// Refresh asks htmx to reload the page.
func Refresh(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Toast queues a toast for the page. It only sets the header, so a handler
// can follow it with a body or a Redirect.
func Toast(w http.ResponseWriter, msg string) {
	trigger(w, map[string]any{events.Toast: msg})
}

// CloseModal dismisses #content-modal, which is how a form inside it reports
// that its save went through. A failure re-renders the form with its errors and
// sends nothing, so the dialog stays open on the thing that needs fixing.
//
// It goes through trigger rather than setting HX-Trigger directly, because a
// handler almost always queues a Toast beside it and a bare Set would drop
// whichever was written first.
func CloseModal(w http.ResponseWriter) {
	trigger(w, map[string]any{events.ModalClose: true})
}

// Theme repaints the page the reader is already on after they change the
// setting. palette is the DaisyUI theme name, or the empty string for system --
// prefs.Theme.Palette answers both.
//
// This is the one thing on the settings form that cannot wait for the next
// navigation. The theme is an attribute on <html>, rendered by the shell, and
// the response to the save swaps a fragment inside a dialog; without this the
// reader picks Dark, the dialog closes, and the page they are looking at stays
// light until they click something. An HX-Refresh instead would be correct and
// would also throw away the toast and scroll position for a setting change.
//
// The dates already on screen are left in the old zone and format, deliberately.
// Rewriting them would mean re-rendering the page, which is the refresh this
// avoids -- and unlike the theme they are not what the reader is looking at
// while they close the dialog.
//
// The detail is an object rather than the bare string, which is not decoration:
// htmx wraps a non-object trigger value as {value: ...} and passes an object
// through as it stands, so a bare string would arrive as e.detail.value while
// every other event this app raises reads a named field. An empty palette also
// survives an object and would be indistinguishable from an absent one after
// the wrap.
func Theme(w http.ResponseWriter, palette string) {
	trigger(w, map[string]any{
		events.ThemeChange: map[string]string{"palette": palette},
	})
}

// Settings hands the page back the settings it is already obeying, after they
// have been saved from a dialog that floated over it.
//
// IT IS Theme's ARGUMENT APPLIED TO THE REST OF THE FORM. The response to a save
// swaps a fragment inside a dialog, so everything OUTSIDE that dialog is still
// running on the answers it was rendered with -- and three of those answers are
// being obeyed right now by something the reader is looking at. The greeting on
// the homepage carries the old name. A tabletop is following the turn, or not
// following it, on the strength of an attribute rendered before the reader
// changed their mind. And it is drawing blood, or not, on the same.
//
// THE OTHER FOUR ARE NOT HERE AND DO NOT WANT TO BE. The zone, the date order
// and the clock are read when a timestamp is rendered, and nothing re-renders
// the timestamps already on the page -- that is a reload, which is the thing
// this whole mechanism exists to avoid. The theme has its own event because it
// is one attribute on <html> that every page carries and no component owns.
//
// NOBODY HAS TO BE LISTENING. This raises one event on window; a page with no
// greeting and no tabletop hears it and does nothing, which is what let the
// settings dialog be opened from the room's Help menu without the room growing
// a copy of the homepage's markup to be swapped into. That copy is what this
// replaced: an out-of-band <span id="account-name"> that only one page in the
// app has ever had an element for.
//
// The detail is an object for the reason Theme's is: htmx passes an object
// through as it stands and wraps anything else as {value: ...}.
func Settings(w http.ResponseWriter, name string, followTurn, showBlood bool, pingVolume int) {
	trigger(w, map[string]any{
		events.SettingsChange: map[string]any{
			"name":       name,
			"followTurn": followTurn,
			"showBlood":  showBlood,
			"pingVolume": pingVolume,
		},
	})
}
