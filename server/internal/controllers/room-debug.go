package controllers

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/a-h/templ"
)

func (a *App) RoomDebugRendererFragment(w http.ResponseWriter, r *http.Request) {
	a.debugFragment(w, r, pages.RoomDebugRenderer())
}
func (a *App) RoomDebugEventsFragment(w http.ResponseWriter, r *http.Request) {
	a.debugFragment(w, r, pages.RoomDebugEvents())
}
func (a *App) RoomDebugStateFragment(w http.ResponseWriter, r *http.Request) {
	a.debugFragment(w, r, pages.RoomDebugState())
}
func (a *App) RoomDebugServerFragment(w http.ResponseWriter, r *http.Request) {
	if !a.Config.Development() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data := pages.RoomDebugServerData{RoomID: row.ID.String()}
	if view, ok := a.Hub.Debug(ctx, row.ID); ok {
		data.Live = true
		data.Groups = debugGroups(view)
		data.PerUser = debugConns(view)
		data.Coalescing = view.Coalescing
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomDebugServer(data))
}
func (a *App) debugFragment(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if !a.Config.Development() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, c)
}
func debugConns(view *hub.DebugView) []pages.RoomDebugConn {
	out := make([]pages.RoomDebugConn, 0, len(view.PerUser))
	for _, conn := range view.PerUser {
		out = append(out, pages.RoomDebugConn{
			User:  conn.User,
			Count: conn.Count,
			Over:  conn.Count >= view.Limits.ConnsPerUser,
		})
	}
	return out
}
func debugGroups(view *hub.DebugView) []pages.RoomDebugGroup {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return []pages.RoomDebugGroup{
		{Label: "Actor", Rows: []pages.RoomDebugRow{
			{Label: "Loaded for", Value: since(view.StartedAt)},
			{Label: "Empty for", Value: emptyFor(view)},
			{Label: "Connections", Value: fmt.Sprintf("%d of %d", view.Conns, view.Limits.ConnsPerRoom),
				Warn: view.Conns >= view.Limits.ConnsPerRoom},
			{Label: "Inbox", Value: fmt.Sprintf("%d of %d", view.Inbox, view.InboxCap),
				Warn: view.Inbox*2 >= view.InboxCap},
			{Label: "Kick grace", Value: fmt.Sprintf("%d held", view.Kicked)},
			{Label: "Rooms here", Value: fmt.Sprintf("%d loaded", view.Loaded)},
		}},
		{Label: "Sequence", Rows: []pages.RoomDebugRow{
			{Label: "GM", Value: fmt.Sprintf("%d", view.SeqGM)},
			{Label: "Player", Value: fmt.Sprintf("%d", view.SeqPlayer)},
			{Label: "Drift", Value: drift(view)},
			{Label: "Changes", Value: fmt.Sprintf("%d applied", view.Changes)},
		}},
		{Label: "Persistence", Rows: []pages.RoomDebugRow{
			{Label: "Unsaved", Value: yesNo(view.Dirty), Warn: view.Dirty},
			{Label: "Saving", Value: yesNo(view.Saving)},
			{Label: "Failures", Value: fmt.Sprintf("%d in a row", view.SaveFailures), Warn: view.SaveFailures > 0},
			{Label: "Last save", Value: lastSave(view)},
			{Label: "Snapshot", Value: snapshotSize(view), Warn: view.SnapshotBytes > view.SoftLimit},
			{Label: "Every", Value: view.Limits.SnapshotInterval.String()},
			{Label: "Unload after", Value: view.Limits.UnloadGrace.String()},
		}},
		{Label: "Contents", Rows: []pages.RoomDebugRow{
			{Label: "Players", Value: fmt.Sprintf("%d", view.Players)},
			{Label: "Pawns", Value: fmt.Sprintf("%d", view.Pawns)},
			{Label: "Floors", Value: fmt.Sprintf("%d", view.Layers)},
			{Label: "Fog", Value: fmt.Sprintf("%d", view.Fog)},
			{Label: "Strokes", Value: fmt.Sprintf("%d", view.Strokes)},
		}},
		{Label: "Limits", Rows: []pages.RoomDebugRow{
			{Label: "Coalesce", Value: view.Limits.CoalesceInterval.String()},
			{Label: "Per user", Value: fmt.Sprintf("%d connections", view.Limits.ConnsPerUser)},
			{Label: "Commands", Value: fmt.Sprintf("%d/s, burst %d", view.Limits.Rate, view.Limits.Burst)},
			{Label: "Over budget", Value: fmt.Sprintf("%d in %s closes it", view.Limits.Overs, view.Limits.OverWindow)},
			{Label: "Read limit", Value: fmt.Sprintf("%d KB", view.Limits.ReadLimit/1024)},
		}},
		{Label: "Process", Rows: []pages.RoomDebugRow{
			{Label: "Goroutines", Value: fmt.Sprintf("%d", runtime.NumGoroutine())},
			{Label: "Heap", Value: fmt.Sprintf("%.1f MB", float64(memory.HeapAlloc)/(1<<20))},
			{Label: "Collections", Value: fmt.Sprintf("%d", memory.NumGC)},
		}},
	}
}
func drift(view *hub.DebugView) string {
	if view.SeqGM >= view.SeqPlayer {
		return fmt.Sprintf("%d ahead for the GM", view.SeqGM-view.SeqPlayer)
	}
	return fmt.Sprintf("%d ahead for players", view.SeqPlayer-view.SeqGM)
}
func snapshotSize(view *hub.DebugView) string {
	if view.SnapshotBytes == 0 {
		return "never encoded"
	}
	share := float64(view.SnapshotBytes) / float64(view.SoftLimit) * 100
	return fmt.Sprintf("%.1f KB, %.1f%% of the ceiling", float64(view.SnapshotBytes)/1024, share)
}
func lastSave(view *hub.DebugView) string {
	if view.SavedAt.IsZero() {
		return "not since it loaded"
	}
	return since(view.SavedAt) + " ago"
}
func emptyFor(view *hub.DebugView) string {
	if view.EmptySince.IsZero() {
		return "somebody is here"
	}
	return since(view.EmptySince)
}
func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
func since(at time.Time) string {
	return time.Since(at).Truncate(time.Second).String()
}
