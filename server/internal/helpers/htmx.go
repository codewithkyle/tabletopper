package helpers

import "net/http"

func HTMXServerError(w http.ResponseWriter) {
	w.Header().Set(
		"HX-Trigger",
		`{"alert":{"heading":"Server Error","message":"Something went wrong on the server. If this continues to happen submit an issue on GitHub."}}`,
	)
	w.WriteHeader(http.StatusInternalServerError)
}

func HTMXCustomError(w http.ResponseWriter, heading string, msg string, statusCode int) {
	w.Header().Set(
		"HX-Trigger",
		`{"alert":{"heading":"`+heading+`","message":"`+msg+`"}}`,
	)
	w.WriteHeader(statusCode)
}

func HTMXUserError(w http.ResponseWriter, heading string, msg string) {
	w.Header().Set(
		"HX-Trigger",
		`{"alert":{"heading":"`+heading+`","message":"`+msg+`"}}`,
	)
	w.WriteHeader(http.StatusBadRequest)
}

func RedirectToError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/error", http.StatusSeeOther)
}

func RedirectToSignIn(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}

func Redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func HTMXRedirect(w http.ResponseWriter, path string) {
	w.Header().Set(
		"HX-Redirect",
		path,
	)
	w.WriteHeader(http.StatusOK)
}

func HTMXRefresh(w http.ResponseWriter) {
	w.Header().Set(
		"HX-Refresh",
		"true",
	)
	w.WriteHeader(http.StatusOK)
}

func HTMXToast(w http.ResponseWriter, msg string) {
	w.Header().Set(
		"HX-Trigger",
		`{"flash:toast":"`+msg+`"}`,
	)
}
