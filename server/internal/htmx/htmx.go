// Package htmx writes the response headers htmx reads: HX-Trigger for the
// alert and toast events the page listens for, HX-Redirect and HX-Refresh
// for navigation. A handler answering an htmx request uses these instead of
// a 3xx, which fetch() would follow and swap into the page.
package htmx

import "net/http"

// ServerError is the generic 500 for an htmx request: an alert event and no
// body, which the noSwap config in base.templ leaves alone.
func ServerError(w http.ResponseWriter) {
	w.Header().Set(
		"HX-Trigger",
		`{"alert":{"heading":"Server Error","message":"Something went wrong on the server. If this continues to happen submit an issue on GitHub."}}`,
	)
	w.WriteHeader(http.StatusInternalServerError)
}

// Error raises an alert with the given heading and message under status.
func Error(w http.ResponseWriter, heading string, msg string, status int) {
	w.Header().Set(
		"HX-Trigger",
		`{"alert":{"heading":"`+heading+`","message":"`+msg+`"}}`,
	)
	w.WriteHeader(status)
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
	w.Header().Set(
		"HX-Trigger",
		`{"flash:toast":"`+msg+`"}`,
	)
}
