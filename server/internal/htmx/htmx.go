



package htmx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"tabletopper/internal/events"
)





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




func Error(w http.ResponseWriter, heading string, msg string, status int) {
	trigger(w, map[string]any{
		events.Alert: map[string]string{"heading": heading, "message": msg},
	})
	w.WriteHeader(status)
}


func ServerError(w http.ResponseWriter) {
	Error(w, "Server Error", "Something went wrong on the server. If this continues to happen submit an issue on GitHub.", http.StatusInternalServerError)
}



func NotFound(w http.ResponseWriter, what string) {
	Error(w, "Not Found", "That "+what+" no longer exists. Refresh the page and try again.", http.StatusNotFound)
}



func Redirect(w http.ResponseWriter, path string) {
	w.Header().Set("HX-Redirect", path)
	w.WriteHeader(http.StatusOK)
}


func Refresh(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}



func Toast(w http.ResponseWriter, msg string) {
	trigger(w, map[string]any{events.Toast: msg})
}








func CloseModal(w http.ResponseWriter) {
	trigger(w, map[string]any{events.ModalClose: true})
}























func Theme(w http.ResponseWriter, palette string) {
	trigger(w, map[string]any{
		events.ThemeChange: map[string]string{"palette": palette},
	})
}



























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
