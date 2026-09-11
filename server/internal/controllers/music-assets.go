package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/audio"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

































const (
	
	
	
	
	
	
	
	
	
	
	maxMusicBytes = 256 << 20 

	
	
	
	
	musicUploadTTL = time.Hour

	
	
	
	
	
	
	
	musicPlaybackTTL = time.Hour
)

func (a *App) MusicAssetsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	tracks, err := a.musicList(ctx, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load music library", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.MusicAssets(tracks))
}









func (a *App) musicList(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MusicTrack, error) {
	var rows []queries.Asset
	var err error

	if term == "" {
		rows, err = a.Queries.GetMusicLibrary(ctx, ownerID)
	} else {
		rows, err = a.Queries.SearchMusicLibrary(ctx, queries.SearchMusicLibraryParams{
			OwnerID: ownerID,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}

	tracks := make([]pages.MusicTrack, 0, len(rows))
	for _, row := range rows {
		tracks = append(tracks, pages.MusicTrack{
			ID:       row.ID.String(),
			Name:     row.Name,
			FileName: row.FileName,
		})
	}

	return tracks, nil
}

func (a *App) RenameMusic(w http.ResponseWriter, r *http.Request) { a.renameLibrary(w, r, musicKind) }
func (a *App) DeleteMusic(w http.ResponseWriter, r *http.Request) { a.deleteLibrary(w, r, musicKind) }




type startUploadRequest struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}


type startUploadResponse struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}









type jsonProblem struct {
	Heading string `json:"heading"`
	Message string `json:"message"`
}

func writeJSONProblem(w http.ResponseWriter, status int, heading string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(jsonProblem{Heading: heading, Message: message}); err != nil {
		slog.Error("Failed to write a JSON problem", "error", err)
	}
}












func (a *App) StartMusicUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	var req startUploadRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSONProblem(w, http.StatusBadRequest, "Upload Failed", "The browser sent something the server could not read. Refresh the page and try again.")
		return
	}

	name := assetName(req.Name)
	contentType, ok := audio.TypeForName(name)
	if !ok {
		writeJSONProblem(w, http.StatusUnsupportedMediaType, "Unsupported Audio",
			"Music must be an MP3, OGG, Opus, M4A, WebM, FLAC or WAV file.")
		return
	}
	if req.Size <= 0 {
		writeJSONProblem(w, http.StatusBadRequest, "Upload Failed", "That file is empty.")
		return
	}
	if req.Size > maxMusicBytes {
		writeJSONProblem(w, http.StatusRequestEntityTooLarge, "Track Too Large",
			"Tracks must be 256 MB or smaller.")
		return
	}

	
	
	
	assetID := ulid.Make()
	key := storage.MusicKey(sess.UserID, assetID)

	err := a.Queries.InsertMusic(ctx, queries.InsertMusicParams{
		ID:       assetID,
		OwnerID:  sess.UserID,
		FilePath: key,
		FileName: name,
		Name:     name,
	})
	if err != nil {
		slog.Error("Failed to insert music row", "error", err)
		writeJSONProblem(w, http.StatusInternalServerError, "Server Error", "Something went wrong on the server. Try again in a moment.")
		return
	}

	url, err := a.Storage.PresignPut(ctx, key, contentType, req.Size, musicUploadTTL)
	if err != nil {
		slog.Error("Failed to presign a music upload", "error", err, "assetID", assetID.String())
		
		
		
		
		
		a.discardAssetRow(ctx, sess.UserID, assetID)
		writeJSONProblem(w, http.StatusInternalServerError, "Server Error", "Something went wrong on the server. Try again in a moment.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(startUploadResponse{
		ID:          assetID.String(),
		URL:         url,
		ContentType: contentType,
	}); err != nil {
		slog.Error("Failed to write a music upload URL", "error", err)
	}
}


























func (a *App) ConfirmMusicUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, ok := a.musicTrack(w, r)
	if !ok {
		return
	}
	if row.UploadedAt.Valid {
		
		
		w.WriteHeader(http.StatusNoContent)
		return
	}

	size, err := a.Storage.Size(ctx, row.FilePath)
	if errors.Is(err, storage.ErrNotFound) {
		a.discardTrack(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Upload Failed", "The track did not finish uploading. Try again.", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("Failed to read an uploaded track", "error", err, "assetID", row.ID.String())
		htmx.Error(w, "Upload Not Confirmed", "The track could not be checked. Refresh the page and try again.", http.StatusBadGateway)
		return
	}
	if size > maxMusicBytes {
		
		
		
		slog.Warn("An uploaded track is over the cap", "assetID", row.ID.String(), "size", size)
		a.discardTrack(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Track Too Large", "Tracks must be 256 MB or smaller.", http.StatusRequestEntityTooLarge)
		return
	}

	
	
	
	head, err := a.Storage.Peek(ctx, row.FilePath, audio.HeaderBytes)
	if errors.Is(err, storage.ErrNotFound) {
		a.discardTrack(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Upload Failed", "The track did not finish uploading. Try again.", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("Failed to read an uploaded track's header", "error", err, "assetID", row.ID.String())
		htmx.Error(w, "Upload Not Confirmed", "The track could not be checked. Refresh the page and try again.", http.StatusBadGateway)
		return
	}

	declared, _ := audio.TypeForName(row.FileName)
	actual, ok := audio.TypeForBytes(head)
	if !ok || actual != declared {
		slog.Warn("An uploaded track is not what its name claims",
			"assetID", row.ID.String(), "declared", declared, "actual", actual)
		a.discardTrack(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Unsupported Audio",
			"That file is not the kind of audio its name says it is. Music must be an MP3, OGG, Opus, M4A, WebM, FLAC or WAV file.",
			http.StatusUnsupportedMediaType)
		return
	}

	
	
	
	result, err := a.Queries.FinishMusicUpload(ctx, queries.FinishMusicUploadParams{
		ID:      row.ID,
		OwnerID: sess.UserID,
		
		
		
		SizeBytes: size,
	})
	if err != nil {
		slog.Error("Failed to finish a music upload", "error", err, "assetID", row.ID.String())
		htmx.ServerError(w)
		return
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	htmx.Toast(w, row.Name+" uploaded.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MusicCard(pages.MusicTrack{
		ID:       row.ID.String(),
		Name:     row.Name,
		FileName: row.FileName,
	}))
}





























func (a *App) GetMusicAudio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	row, err := a.Queries.GetMusicTrack(ctx, queries.GetMusicTrackParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load a track", "error", err, "assetID", assetID.String())
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}
	
	if !row.UploadedAt.Valid {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	url, err := a.Storage.PresignGet(ctx, row.FilePath, musicPlaybackTTL)
	if err != nil {
		slog.Error("Failed to presign a track", "error", err, "assetID", assetID.String())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, url, http.StatusFound)
}



func (a *App) musicTrack(w http.ResponseWriter, r *http.Request) (queries.Asset, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, musicKind.One)
		return queries.Asset{}, false
	}

	row, err := a.Queries.GetMusicTrack(ctx, queries.GetMusicTrackParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, musicKind.One)
		return queries.Asset{}, false
	}
	if err != nil {
		slog.Error("Failed to load a track", "error", err)
		htmx.ServerError(w)
		return queries.Asset{}, false
	}

	return row, true
}









func (a *App) discardTrack(ctx context.Context, userID, assetID ulid.ULID, key string) {
	a.discardAsset(ctx, userID, assetID, func(c context.Context) error {
		return a.Storage.Delete(c, key)
	})
}
