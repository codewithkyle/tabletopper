package controllers

import (
	"net/http"

	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
)

func (a *App) RoomTableMenuFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	isGM := role == room.RoleGM
	var ring []pages.RoomTableMenuArt
	if a.Hub != nil {
		if view, ok := a.Hub.Table(ctx, row.ID); ok {
			ring = tableMenuRingFor(isGM, view.Table)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomTableMenu(pages.NewTableMenu(row.ID.String(), isGM, ring)))
}
func tableMenuRingFor(isGM bool, t room.Table) []pages.RoomTableMenuArt {
	if !isGM && !t.PlayersCanStamp {
		return nil
	}
	return tableMenuRing(t.Palette)
}
func tableMenuRing(palette []room.TileArt) []pages.RoomTableMenuArt {
	out := make([]pages.RoomTableMenuArt, 0, len(palette))
	for i, art := range palette {
		out = append(out, pages.RoomTableMenuArt{
			ID:    art.ID.String(),
			Name:  art.Name,
			Image: art.Image,
			Index: i,
			Count: len(palette),
		})
	}
	return out
}
