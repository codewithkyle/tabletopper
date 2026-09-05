// Package htmx writes the response headers htmx reads: HX-Trigger for the
// alert and toast events the page listens for, HX-Redirect and HX-Refresh
// for navigation. A handler answering an htmx request uses these instead of
// a 3xx, which fetch() would follow and swap into the page.
package htmx

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
		"alert": map[string]string{"heading": heading, "message": msg},
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
	trigger(w, map[string]any{"flash:toast": msg})
}
