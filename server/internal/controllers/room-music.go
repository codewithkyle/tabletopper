package controllers

import (
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomMusicFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	view, ok := a.Hub.Music(ctx, row.ID)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data := musicData(row.ID, role, view.Music)
	if data.CanControl {
		tracks, err := a.musicList(ctx, sess.UserID, "")
		if err != nil {
			slog.Error("Failed to list music for a room", "error", err, "room", row.ID.String())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data.Tracks = tracks
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMusic(data))
}
func (a *App) RoomMusicLibraryFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || role != room.RoleGM {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	tracks, err := a.musicList(ctx, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to search music for a room", "error", err, "room", row.ID.String())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMusicList(pages.RoomMusicData{
		RoomID:     row.ID.String(),
		CanControl: true,
		Query:      term,
		Tracks:     tracks,
	}))
}
func (a *App) LoadRoomMusic(w http.ResponseWriter, r *http.Request) {
	track, ok := a.musicTrackField(w, r)
	if !ok {
		return
	}
	a.musicCommand(w, r, "load music", &room.MusicLoad{AssetID: track})
}
func (a *App) PlayRoomMusic(w http.ResponseWriter, r *http.Request) {
	a.musicCommand(w, r, "start the music", &room.MusicPlay{})
}
func (a *App) PauseRoomMusic(w http.ResponseWriter, r *http.Request) {
	a.musicCommand(w, r, "pause the music", &room.MusicPause{})
}
func (a *App) StopRoomMusic(w http.ResponseWriter, r *http.Request) {
	a.musicCommand(w, r, "stop the music", &room.MusicStop{})
}
func (a *App) LoopRoomMusic(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	a.musicCommand(w, r, "repeat the music", &room.MusicSetLoop{Loop: r.FormValue("loop") != ""})
}
func (a *App) EndRoomMusic(w http.ResponseWriter, r *http.Request) {
	track, ok := a.musicTrackField(w, r)
	if !ok {
		return
	}
	a.musicCommand(w, r, "end the track", &room.MusicEnded{TrackID: track})
}
func (a *App) RoomMusicAudio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	view, ok := a.Hub.Music(ctx, row.ID)
	if !ok || view.Music.TrackID == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	asked, err := ulid.Parse(r.URL.Query().Get("track"))
	if err != nil || asked != *view.Music.TrackID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	asset, err := a.Queries.GetMusicTrack(ctx, queries.GetMusicTrackParams{ID: asked, OwnerID: row.OwnerID})
	if err != nil || !asset.UploadedAt.Valid {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	url, err := a.Storage.PresignGet(ctx, asset.FilePath, musicPlaybackTTL)
	if err != nil {
		slog.Error("Failed to presign a room's track", "error", err, "assetID", asked.String())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, url, http.StatusFound)
}
func (a *App) musicCommand(w http.ResponseWriter, r *http.Request, action string, cmd room.Command) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}
	if err := a.Hub.Dispatch(r.Context(), roomID, who, cmd); err != nil {
		a.rejectCommand(w, action, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) musicTrackField(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return ulid.ULID{}, false
	}
	track, err := ulid.Parse(strings.TrimSpace(r.FormValue("track")))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return ulid.ULID{}, false
	}
	return track, true
}
func musicData(roomID ulid.ULID, role room.Role, music room.Music) pages.RoomMusicData {
	return pages.RoomMusicData{
		RoomID:     roomID.String(),
		CanControl: role == room.RoleGM,
		Loaded:     music.TrackID != nil,
		Name:       music.Name,
		Playing:    music.Playing,
		Loop:       music.Loop,
		Started:    music.At > 0,
	}
}
