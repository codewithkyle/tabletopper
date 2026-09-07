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

// MUSIC IS THE ONE KIND WHOSE BYTES NEVER PASS THROUGH THIS PROCESS, and every
// odd thing in this file follows from that one decision.
//
// A track is an hour or two long -- battle music, a tavern, an hour of rain --
// which is 115 to 175 MB. Through the multipart path every other upload uses,
// that is a 175 MB allocation, the same again spilled to the container's /tmp,
// a read deadline that amounts to demanding 2.3 Mbps of whoever is uploading,
// and then all of it sent a second time from here to R2. So the browser PUTs
// straight to the bucket through a presigned URL.
//
// THAT MAKES AN UPLOAD TWO REQUESTS WITH A GAP, and the gap is where the care
// goes:
//
//	POST /assets/music          -- writes the row, answers with a signed URL
//	  (the browser PUTs to R2, for as long as that takes)
//	POST /assets/music/{id}/confirm -- checks the object, finishes the row
//
// The row is written first because the row is the ledger for what lives in R2:
// a presigned URL names a key, and no key may exist that no row claims. Between
// the two, uploaded_at is NULL and the row owns a key and nothing else -- it is
// not listed, it is not playable, and internal/sweep collects it if the confirm
// never comes.
//
// THE CONFIRM IS NOT A FORMALITY. It is the only place the file is ever
// inspected, because the handler that would have inspected it never saw it: it
// reads the object's size and its first 64 bytes back out of the bucket and
// rolls the whole upload back if either is wrong.
//
// NOTHING HERE IS SHARED WITH library-assets.go except renaming and deleting,
// which do not care what an asset is made of. Music has no decode, no resize, no
// dimensions and no encoder in common with a picture.

const (
	// maxMusicBytes is the largest track that may be uploaded, and it is set by
	// what these files actually are rather than by what feels tidy: two hours
	// at 192 kbps is about 173 MB, and 256 leaves room above that without
	// inviting somebody's lossless archive.
	//
	// IT IS ENFORCED BY THE SIGNATURE, NOT BY A READER. The bytes never reach a
	// handler that could count them, so the size the browser declares is signed
	// into the presigned URL as Content-Length and R2 refuses a body that is
	// not exactly that long. The confirm checks it again afterwards, because a
	// cap that is only enforced by something else is a cap on trust.
	maxMusicBytes = 256 << 20 // 256 MiB

	// musicUploadTTL is how long a signed PUT stays good. It has to cover the
	// whole upload, because the signature is checked when the request is made
	// and the request is the upload -- 175 MB at 2 Mbps is a little over ten
	// minutes, and this is generous over that.
	musicUploadTTL = time.Hour

	// musicPlaybackTTL is how long a signed GET stays good.
	//
	// IT DOES NOT HAVE TO OUTLAST THE TRACK. Every player goes through the
	// redirect below, which mints a fresh URL for each request the audio
	// element makes -- including each range request it makes to seek -- so a
	// URL is used once, immediately. What this covers is the gap between the
	// redirect and the request that follows it.
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

// musicList is the music shelf in either of its two states -- every finished
// track, or the ones that matched a search -- so the page and the search
// fragment build the same cards from the same function.
//
// BOTH STATEMENTS DROP THE ROWS WHOSE UPLOAD NEVER FINISHED, and the search one
// has to say so for itself: uploaded_at IS NOT NULL is in the WHERE of each.
// Without it, typing a letter of an abandoned upload's name would put a card on
// the page for a track that is not in the bucket.
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

// startUploadRequest is what the browser sends to begin an upload: the file it
// is about to send, described. There is no body beyond this -- the file itself
// goes to R2.
type startUploadRequest struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// startUploadResponse is the signed URL and what to send with it.
type startUploadResponse struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}

// jsonProblem is an error for a caller that is fetch() rather than htmx.
//
// IT IS THE SAME SHAPE AS THE ALERT'S HX-Trigger DETAIL, deliberately: the two
// music routes that answer JSON cannot use htmx.Error, because a plain fetch
// does not read response headers looking for events -- so the body carries what
// the header would have, and public/js/music-upload.js dispatches the same
// "alert" event the modal already listens for. One dialog, one shape, two ways
// of getting there.
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

// StartMusicUpload claims a key and hands back a URL the browser may PUT to.
//
// EVERYTHING IS CHECKED BEFORE ANYTHING IS WRITTEN, because everything it can
// check is in this request: the name says what format the file claims to be,
// and the size says how big it claims to be. A refusal here costs the uploader
// nothing -- no row, no URL, and the file never leaves their machine.
//
// THE SIZE IS TAKEN FROM THE BROWSER AND THEN MADE BINDING. It arrives as a
// number a caller chose, which is worth nothing on its own; signing it into the
// URL as Content-Length is what turns it into a promise R2 enforces. A caller
// who declares 1 MB and sends 200 gets a rejection from the bucket.
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

	// The row goes first and names the key the object will land at. It is the
	// ledger for what lives in R2, and an object under a key no row claims is a
	// file nothing can play, no delete will find and no sweep will collect.
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
		// The row named a key nothing was ever given a way to write to, so it
		// is dropped rather than left for the sweep -- there is no object to
		// tidy and no window in which one could appear.
		a.discardMusicRow(ctx, sess.UserID, assetID)
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

// ConfirmMusicUpload is the second half, and the only place a track is ever
// inspected.
//
// IT ASKS THE BUCKET RATHER THAN THE BROWSER. The browser has just told us it
// finished, which is not evidence; a HEAD says whether an object is there and
// how big it is, and 64 bytes read back says what it actually is. Both are one
// small request against R2 whatever the size of the track.
//
// THE NAME IS NOT TAKEN AT ITS WORD. The Content-Type was signed from the
// filename, because that was all that was known at signing time, and R2 stores
// and serves whatever it was given -- so a WebM called track.mp3 would be served
// as audio/mpeg forever. Comparing the sniffed type against the one the name
// implied is what closes that.
//
// EVERY FAILURE ROLLS THE WHOLE UPLOAD BACK, object first and row last, which is
// the order every delete in this app uses.
func (a *App) ConfirmMusicUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, ok := a.musicTrack(w, r)
	if !ok {
		return
	}
	if row.UploadedAt.Valid {
		// Already confirmed. The card is on the page, so answering with a
		// second one would draw the track twice.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	size, err := a.Storage.Size(ctx, row.FilePath)
	if err != nil {
		slog.Error("Failed to read an uploaded track", "error", err, "assetID", row.ID.String())
		a.discardMusic(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Upload Failed", "The track did not finish uploading. Try again.", http.StatusBadGateway)
		return
	}
	if size > maxMusicBytes {
		// The signature should have made this impossible, which is exactly why
		// it is checked: a cap enforced only by something else is a cap on
		// trust in that something else.
		slog.Warn("An uploaded track is over the cap", "assetID", row.ID.String(), "size", size)
		a.discardMusic(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Track Too Large", "Tracks must be 256 MB or smaller.", http.StatusRequestEntityTooLarge)
		return
	}

	head, err := a.Storage.Peek(ctx, row.FilePath, audio.HeaderBytes)
	if err != nil {
		slog.Error("Failed to read an uploaded track's header", "error", err, "assetID", row.ID.String())
		a.discardMusic(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Upload Failed", "The track could not be read back. Try again.", http.StatusBadGateway)
		return
	}

	declared, _ := audio.TypeForName(row.FileName)
	actual, ok := audio.TypeForBytes(head)
	if !ok || actual != declared {
		slog.Warn("An uploaded track is not what its name claims",
			"assetID", row.ID.String(), "declared", declared, "actual", actual)
		a.discardMusic(ctx, sess.UserID, row.ID, row.FilePath)
		htmx.Error(w, "Unsupported Audio",
			"That file is not the kind of audio its name says it is. Music must be an MP3, OGG, Opus, M4A, WebM, FLAC or WAV file.",
			http.StatusUnsupportedMediaType)
		return
	}

	// uploaded_at IS NULL is in this statement's WHERE, so two confirms write
	// once. Zero rows means another one got here first, and the card it
	// answered with is already on the page.
	result, err := a.Queries.FinishMusicUpload(ctx, queries.FinishMusicUploadParams{
		ID:      row.ID,
		OwnerID: sess.UserID,
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

// GetMusicAudio sends the player to the bucket.
//
// IT IS A REDIRECT AND NOT A PROXY, and that is what makes seeking work. An
// <audio> element plays by asking for byte ranges, and Safari will not play a
// source that cannot answer one; R2 answers them natively, while answering them
// here would mean implementing 206 and Content-Range and streaming every
// listener's copy of a 175 MB track through this process for the length of a
// session.
//
// A BROWSER REPEATS THE REQUEST'S METHOD AND HEADERS THROUGH A 302, so the Range
// header the audio element set survives the hop and arrives at R2 intact. That
// is the whole mechanism.
//
// IT ALSO SOLVES EXPIRY, which is the reason there is no JSON route handing URLs
// to a script that would have to notice a 403 and re-mint. Every request the
// player makes -- the first one, and every seek after it -- comes back through
// here and gets a URL minted a moment earlier. no-store is what keeps it that
// way: a cached redirect would hand out a signature that had gone stale.
//
// It is owner-scoped, which the image routes deliberately are not. Those serve
// pictures every player at a table can see; nothing but this account's own
// manager can reach a track yet, and the day a room needs to play one to
// everybody in it, that is a room's question rather than a reason to open this
// to every signed-in user now.
//
// RequireSessionOr404, for the reason the image routes are: this is the src of a
// media element, and a redirect to the sign-in page renders as a player that
// will not play rather than as a sign-in page.
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
	// A row whose upload never finished names a key with nothing behind it.
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

// musicTrack parses the id and loads the row, scoped to the owner and to music.
// It writes the response itself when there is nothing to find.
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

// discardMusic rolls back an upload whose object landed and was refused: the
// object goes first and the row only once R2 has confirmed it is gone, which is
// the order every delete here uses.
func (a *App) discardMusic(ctx context.Context, userID ulid.ULID, assetID ulid.ULID, key string) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Storage.Delete(cleanupCtx, key); err != nil {
		slog.Error("Failed to clean up a refused track; leaving the row for the sweep", "error", err, "assetID", assetID.String())
		return
	}
	if err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{ID: assetID, OwnerID: userID}); err != nil {
		slog.Error("Failed to delete a refused track's row", "error", err, "assetID", assetID.String())
	}
}

// discardMusicRow drops a row that never got as far as owning anything. It is
// separate from discardMusic because there is nothing in the bucket to delete
// and no reason to ask: the presign failed, so no URL was ever handed out.
func (a *App) discardMusicRow(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{ID: assetID, OwnerID: userID}); err != nil {
		slog.Error("Failed to delete an unsigned track's row", "error", err, "assetID", assetID.String())
	}
}
