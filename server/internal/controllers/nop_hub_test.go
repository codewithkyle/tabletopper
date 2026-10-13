package controllers

import (
	"context"
	"net/http"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

type nopHub struct{}

func (nopHub) Dispatch(context.Context, ulid.ULID, room.Actor, room.Command) error {
	return hub.ErrNoRoom
}
func (nopHub) Notify(ulid.ULID, room.Command)                                 {}
func (nopHub) SyncCharacter(context.Context, ulid.ULID, ulid.ULID, ulid.ULID) {}
func (nopHub) Close(context.Context, ulid.ULID)                               {}
func (nopHub) ForgetCharacter(context.Context, ulid.ULID)                     {}
func (nopHub) ForgetMonster(context.Context, ulid.ULID)                       {}
func (nopHub) ForgetAsset(context.Context, ulid.ULID)                         {}
func (nopHub) Table(context.Context, ulid.ULID) (*hub.TableView, bool)        { return nil, false }
func (nopHub) Players(context.Context, ulid.ULID) ([]room.Player, bool)       { return nil, false }
func (nopHub) Pawn(context.Context, ulid.ULID, ulid.ULID, room.Role) (*room.Pawn, bool) {
	return nil, false
}
func (nopHub) CharacterPawn(context.Context, ulid.ULID, ulid.ULID) (*room.Pawn, bool) {
	return nil, false
}
func (nopHub) Initiative(context.Context, ulid.ULID, room.Role) (*hub.InitiativeView, bool) {
	return nil, false
}
func (nopHub) Music(context.Context, ulid.ULID) (*hub.MusicView, bool)            { return nil, false }
func (nopHub) Rolls(context.Context, ulid.ULID, ulid.ULID) (*hub.RollsView, bool) { return nil, false }
func (nopHub) Export(context.Context, ulid.ULID) (*hub.ExportView, bool)          { return nil, false }
func (nopHub) Debug(context.Context, ulid.ULID) (*hub.DebugView, bool)            { return nil, false }
func (nopHub) Autosave(context.Context, ulid.ULID) bool                           { return false }
func (nopHub) Serve(w http.ResponseWriter, _ *http.Request, _ ulid.ULID, _ room.Player, _ hub.Membership) {
	w.WriteHeader(http.StatusNotFound)
}
func (nopHub) Version() string { return "" }
