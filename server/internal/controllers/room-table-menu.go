package controllers

import (
	"net/http"

	"tabletopper/internal/room"
	"tabletopper/templ/pages"
)

func (a *App) RoomTableMenuFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, role, ok := a.member(w, r)
	if !ok {
		return
	}
	isGM := role == room.RoleGM
	var ring []pages.RoomTableMenuArt
	if view, ok := a.Hub.Table(ctx, row.ID); ok {
		ring = tableMenuRingFor(isGM, view.Table)
	}
	renderFragment(w, r, pages.RoomTableMenu(pages.NewTableMenu(row.ID.String(), isGM, ring)))
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
