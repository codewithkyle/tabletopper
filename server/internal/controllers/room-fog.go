package controllers
import (
	"net/http"
	"github.com/oklog/ulid/v2"
	"tabletopper/internal/room"
)
func (a *App) FillLayerFog(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "fill the fog", func(layer ulid.ULID) []room.Command {
		return []room.Command{
			&room.FogClear{Layer: layer},
			&room.FogSetPrefill{Layer: layer, Prefill: true},
			&room.FogSetEnabled{Layer: layer, Enabled: true},
		}
	})
}
func (a *App) ClearLayerFog(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the fog", func(layer ulid.ULID) []room.Command {
		return []room.Command{
			&room.FogClear{Layer: layer},
			&room.FogSetEnabled{Layer: layer, Enabled: false},
		}
	})
}
