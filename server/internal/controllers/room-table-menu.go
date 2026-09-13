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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomTableMenu(pages.RoomTableMenuData{
		RoomID: row.ID.String(),
		Items:  pages.TableMenuItems(role == room.RoleGM),
	}))
}
