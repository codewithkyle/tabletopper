package controllers

import (
	"net/http"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func (a *App) ClearLayerDrawing(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the drawing", func(layer ulid.ULID) []room.Command {
		return []room.Command{&room.StrokeClear{Layer: layer}}
	})
}
