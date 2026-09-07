package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	// Uploads are read with image.DecodeConfig and decoded by imaging, and
	// both know only the formats that registered themselves. PNG and JPEG
	// are registered here rather than relied on from a dependency's
	// imports; webp registers itself in the chai2010 package below.
	_ "image/jpeg"
	_ "image/png"

	"tabletopper/internal/htmx"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/internal/tiling"
	"tabletopper/templ/pages"

	"github.com/disintegration/imaging"
	"github.com/oklog/ulid/v2"
)

const (
	// maxUploadBytes and maxUploadPixels bound an image that is decoded inside
	// the request: an avatar, or a picture pasted into a journal entry.
	//
	// The byte cap is what a person may send. The pixel cap is what those
	// bytes are allowed to expand to, which the byte cap does not bound at
	// all: an 8 MiB PNG can declare a 20,000 by 20,000 canvas and decode to
	// 1.6 GB. Forty megapixels is roughly 160 MB of NRGBA, which is one
	// upload in flight.
	maxUploadBytes  = 8 << 20 // 8 MiB
	maxUploadPixels = 40_000_000

	// maxMapBytes and maxMapPixels are a map's, and they are larger by an
	// order of magnitude for one reason: A MAP IS NEVER DECODED IN A REQUEST.
	// The header pass refuses an oversized canvas and the bytes go to R2
	// exactly as they arrived, so what these bound is a transfer rather than
	// an allocation. 12,000 by 9,000 -- the size this whole tiling
	// arrangement exists for -- is 108 megapixels, which the old cap refused
	// outright.
	//
	// The pixel cap is the tiling worker's memory ceiling instead. It decodes
	// one map at a time and peaks at roughly five bytes per pixel, so 150
	// megapixels is about 750 MB the box has to have.
	//
	// THE TWO PAIRS ARE SEPARATE BECAUSE ONE SHARED CAP WOULD UNDO THE WHOLE
	// POINT: it would let an avatar upload decode 150 megapixels inside a
	// handler, which is the half a gigabyte that was just moved out of the
	// request path.
	maxMapBytes  = 128 << 20 // 128 MiB
	maxMapPixels = 150_000_000

	// multipartMemory is how much of a form is held in memory before the rest
	// spills to a temp file.
	//
	// IT IS NOT THE CAP, though ParseMultipartForm's argument reads like it
	// wants one -- passing maxMapBytes would be a 128 MB heap allocation for
	// a single upload. http.MaxBytesReader is what enforces the cap; this
	// only decides where the bytes live on the way through. Past it the body
	// is a file in the container's /tmp for the life of the request, so there
	// has to be room for one there.
	multipartMemory = 32 << 20 // 32 MiB

	// uploadReadDeadline is how long a request has to deliver an upload, and
	// uploadWriteDeadline is that plus room to answer it.
	//
	// THE SERVER'S GLOBAL TIMEOUTS ARE A BANDWIDTH FLOOR. ReadTimeout covers
	// the body and not just the headers, so five seconds for 128 MiB is a
	// demand for 200 Mbps. WriteTimeout blocks it just as hard and far less
	// obviously: net/http sets that deadline when the request headers are
	// read, as an absolute time, so a sixty-second upload reads to completion
	// and then cannot write its response.
	//
	// Both are lifted per request rather than globally, so every other route
	// keeps the five seconds that stop a connection being held open on a body
	// nobody is sending. Ten minutes covers 128 MiB at around 2 Mbps.
	uploadReadDeadline  = 10 * time.Minute
	uploadWriteDeadline = uploadReadDeadline + 30*time.Second

	avatarSize = 96

	// A monster's picture is stored at 256 and an avatar at 96, and the
	// difference is what each one is for. A portrait is a thumbnail on a card
	// and in a bar, and it is never drawn any larger. A monster's picture is
	// what its pawn will be drawn with on a map, at whatever zoom the GM is
	// working at, so it needs pixels the card does not use.
	monsterImageSize = 256
)

// uploadLimits is what one kind of upload is allowed to be. There are two.
type uploadLimits struct {
	bytes  int64
	pixels int64
}

var (
	// imageLimits is for anything this process decodes; mapLimits for the one
	// thing it does not.
	imageLimits = uploadLimits{bytes: maxUploadBytes, pixels: maxUploadPixels}
	mapLimits   = uploadLimits{bytes: maxMapBytes, pixels: maxMapPixels}
)

// extendUploadDeadlines gives one request longer than the server's global
// timeouts allow.
//
// A deadline set through the ResponseController overrides the one ReadTimeout
// or WriteTimeout established when the request began, which is what makes this
// work without touching either global. It has to happen before the body is
// read: a deadline set after it has already passed does not extend anything.
func extendUploadDeadlines(w http.ResponseWriter) {
	now := time.Now()
	controller := http.NewResponseController(w)

	// A failure here leaves the server's deadlines in place, so a large
	// upload is about to be cut off mid-body with nothing else to explain it.
	// It means something between here and net/http wrapped the ResponseWriter
	// without an Unwrap method.
	if err := controller.SetReadDeadline(now.Add(uploadReadDeadline)); err != nil {
		slog.Error("Failed to extend the upload read deadline", "error", err)
	}
	if err := controller.SetWriteDeadline(now.Add(uploadWriteDeadline)); err != nil {
		slog.Error("Failed to extend the upload write deadline", "error", err)
	}
}

func (a *App) AssetsPage(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/assets/maps")
}

func (a *App) MapAssetsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	maps, err := a.Queries.GetMaps(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to load maps", "error", err)
		redirectToError(w, r)
		return
	}

	cards := make([]pages.MapAsset, 0, len(maps))
	for _, m := range maps {
		cards = append(cards, mapCard(m))
	}

	render(w, r, pages.MapAssets(cards))
}

// mapCard is the assets row as the card reads it: four values out of twenty-one
// columns, and the two that matter flattened into what the markup can ask
// about. See MapAsset in templ/pages/assets.go for why the generation and the
// job's state are kept apart rather than folded into one status.
//
// A NULL tile_state flattens to the empty string, which is neither pending nor
// working nor failed -- so a row from before there was a tiling worker renders
// as a card that does not poll and offers no retry, rather than as one stuck
// waiting for a job nothing will ever run.
func mapCard(m queries.Asset) pages.MapAsset {
	card := pages.MapAsset{
		ID:       m.ID.String(),
		Name:     m.Name,
		FileName: m.FileName,
		State:    m.TileState.AssetsTileState,
	}
	if m.TileGen != nil {
		card.Generation = m.TileGen.String()
	}

	return card
}

// MapCardFragment is the card asking what became of its map. A card whose
// tiling job is queued or running polls this every couple of seconds and swaps
// itself with what comes back, so the poll stops by virtue of what it is
// answered with: a finished card carries no hx-trigger.
//
// It renders pages.MapCard, which is the component the page render uses. A
// fragment is never a second copy of markup.
//
// A FAILURE HERE IS A BARE 404 rather than an alert. This request is a
// background poll that the owner did not make, and an error dialog opening by
// itself over the asset manager would be the card reporting on its own
// housekeeping. htmx leaves the target alone on a 4xx, so the card that is
// already on screen simply stays.
func (a *App) MapCardFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load map card", "error", err, "assetID", assetID.String())
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MapCard(mapCard(m)))
}

// GetImage and GetImagePreview proxy an image out of R2. Both are behind
// RequireSessionOr404 and neither is scoped to the owner: see GetImage in
// assets.sql for why.
func (a *App) GetImage(w http.ResponseWriter, r *http.Request) {
	a.serveImage(w, r, false)
}

func (a *App) GetImagePreview(w http.ResponseWriter, r *http.Request) {
	a.serveImage(w, r, true)
}

// serveImage answers a conditional request from the row alone. The ETag is the
// asset id plus updated_at, which every write to an asset bumps, so a browser
// that has the current bytes gets a 304 without R2 being asked. Cache-Control
// is no-cache, not no-store: the browser keeps the bytes, it just has to ask
// first, which is what lets a replaced avatar show up on the next paint at the
// same URL.
func (a *App) serveImage(w http.ResponseWriter, r *http.Request, preview bool) {
	ctx := r.Context()

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	asset, err := a.Queries.GetImage(ctx, assetID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load image row", "error", err, "assetID", assetID.String())
		}
		http.NotFound(w, r)
		return
	}

	key := asset.FilePath
	if preview && asset.PreviewPath.Valid {
		key = asset.PreviewPath.String
	}

	w.Header().Set("Cache-Control", "private, no-cache")
	a.streamImage(w, r, key, fmt.Sprintf(`"%s-%d"`, assetID, asset.UpdatedAt.Unix()))
}

// streamImage answers a conditional request from the ETag it is given and
// otherwise streams the object out of R2 rather than buffering it. Every route
// that serves image bytes ends here.
//
// THE CALLER SETS Cache-Control BEFORE CALLING, because it is the one header
// they disagree on: an avatar or a map preview can be replaced at its URL and
// has to be revalidated, while a journal image and a map tile cannot be and
// never are. The ETag is the caller's for the same reason.
func (a *App) streamImage(w http.ResponseWriter, r *http.Request, key string, etag string) {
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	body, size, err := a.Storage.Get(r.Context(), key)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err, "key", key)
		// The status and nothing else. Every caller of this function is
		// answering an <img> or a renderer's fetch, and http.NotFound would
		// write "404 page not found" where the bytes were meant to be.
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "image/webp")
	if size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		// Almost always the browser navigating away mid-download.
		slog.Debug("Image stream ended early", "error", err, "key", key)
	}
}

// openImageUpload pulls one image out of a multipart form and checks it from
// its header alone. It writes the response itself when something is wrong with
// the upload, so a caller only has to stop: the false return means "already
// answered". The caller owns the file, which is rewound to the start, and must
// close it.
//
// THE HEADER PASS IS WHAT MAKES THE SIZE REFUSABLE. image.DecodeConfig reads
// the dimensions and stops, so an upload declaring more pixels than the budget
// is answered before a decoder has allocated anything; a check after the decode
// would be a check made from inside the allocation it was meant to prevent.
// multipart.File is an io.Seeker, so whatever the caller does next starts from
// the beginning again.
//
// THE DIMENSIONS ARE CHECKED AND THEN THROWN AWAY. They are the ones in the
// file, and an EXIF rotation tag means those are not the ones the image is
// actually laid out over -- so they are fine for refusing a canvas that is too
// large and wrong for anything that has to be stored.
//
// The format comes from that header pass rather than from the Content-Type the
// browser claimed. imaging registers GIF, BMP and TIFF decoders as a side
// effect of importing it, so decoding alone is not the allowlist --
// DecodeConfig uses the same registered decoders as Decode, so the name it
// reports is the one the allowlist means. It is also what the content type
// returned here is built from, for the same reason.
func openImageUpload(w http.ResponseWriter, r *http.Request, field string, limits uploadLimits) (multipart.File, string, string, bool) {
	extendUploadDeadlines(w)

	r.Body = http.MaxBytesReader(w, r.Body, limits.bytes)
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			htmx.Error(w, "Upload Too Large", fmt.Sprintf("Images must be %d MiB or smaller.", limits.bytes>>20), http.StatusRequestEntityTooLarge)
			return nil, "", "", false
		}
		slog.Error("Failed to parse multipart form", "error", err)
		htmx.Error(w, "Upload Failed", "The upload could not be read. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", "", false
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		slog.Error("Failed to get upload from form", "field", field, "error", err)
		htmx.Error(w, "Upload Failed", "No image was attached. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", "", false
	}
	defer file.Close()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		slog.Warn("Failed to read upload header", "field", field, "error", err)
		unsupportedImage(w)
		return nil, "", "", false
	}
	switch format {
	case "png", "jpeg", "webp":
	default:
		unsupportedImage(w)
		return nil, "", "", false
	}
	// int64, so the multiplication cannot wrap on a declared canvas large
	// enough to try -- the whole point of this check is a header nobody sane
	// wrote.
	if int64(cfg.Width)*int64(cfg.Height) > limits.pixels {
		htmx.Error(w, "Image Too Large", fmt.Sprintf("Images must be %d megapixels or fewer.", limits.pixels/1_000_000), http.StatusRequestEntityTooLarge)
		return nil, "", "", false
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		slog.Error("Failed to rewind upload after reading its header", "field", field, "error", err)
		htmx.Error(w, "Upload Failed", "The upload could not be read. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", "", false
	}

	return file, header.Filename, "image/" + format, true
}

// readImageUpload validates an upload and decodes it. It is what every path
// that resizes, crops or re-encodes an image uses.
//
// THE DECODE IS IMAGING'S RATHER THAN image.Decode, for the orientation tag. A
// photograph taken on a phone records its rotation in EXIF and stores the
// pixels unrotated; image.Decode ignores the tag, so the picture would be
// stored on its side and there is nothing in the app to turn it back. imaging
// applies the tag to a JPEG and leaves every other format untouched. It returns
// no format name, which is the other reason the name comes from the header.
func readImageUpload(w http.ResponseWriter, r *http.Request, field string, limits uploadLimits) (image.Image, string, bool) {
	file, filename, _, ok := openImageUpload(w, r, field, limits)
	if !ok {
		return nil, "", false
	}
	defer file.Close()

	src, err := imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		slog.Warn("Failed to decode upload", "field", field, "error", err)
		unsupportedImage(w)
		return nil, "", false
	}

	return src, filename, true
}

// readImageBytes validates an upload and hands back the bytes exactly as they
// arrived, along with what the header said they are.
//
// A MAP IS NEVER DECODED IN A REQUEST. The header pass has already refused a
// canvas larger than the cap, which is the check that matters, and decoding a
// hundred-megapixel image to re-encode it would put half a gigabyte and most of
// a minute inside a handler. The bytes are stored as they came and the tiling
// worker is what decodes them, once, later.
func readImageBytes(w http.ResponseWriter, r *http.Request, field string, limits uploadLimits) ([]byte, string, string, bool) {
	file, filename, contentType, ok := openImageUpload(w, r, field, limits)
	if !ok {
		return nil, "", "", false
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		slog.Error("Failed to read upload", "field", field, "error", err)
		htmx.Error(w, "Upload Failed", "The upload could not be read. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", "", false
	}

	return body, filename, contentType, true
}

func unsupportedImage(w http.ResponseWriter) {
	htmx.Error(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
}

func (a *App) UploadCharacterAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}

	src, filename, ok := readImageUpload(w, r, "avatar", imageLimits)
	if !ok {
		return
	}
	avatar, err := images.EncodeWebP(images.Square(src, avatarSize))
	if err != nil {
		slog.Error("Failed to encode avatar as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	character, err := a.Queries.GetCharacterAsset(ctx, queries.GetCharacterAssetParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "character")
		return
	}
	if err != nil {
		slog.Error("Failed to get character asset", "error", err)
		htmx.ServerError(w)
		return
	}

	if character.AssetID != nil {
		// NOTE: replacing overwrites the existing key, so nothing can be orphaned
		assetID := *character.AssetID
		if err := a.Storage.UploadAvatar(ctx, sess.UserID, assetID, avatar); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			htmx.ServerError(w)
			return
		}
		err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:       assetID,
			OwnerID:  sess.UserID,
			FileName: filename,
		})
		if err != nil {
			slog.Error("Failed to update avatar asset", "error", err)
			htmx.ServerError(w)
			return
		}
	} else {
		// NOTE: the row is the ledger for what lives in R2, so it is written
		// first and rolled back if the upload never lands
		assetID := ulid.Make()
		err := a.Queries.InsertAvatar(ctx, queries.InsertAvatarParams{
			ID:       assetID,
			OwnerID:  sess.UserID,
			FilePath: storage.AvatarKey(sess.UserID, assetID),
			FileName: filename,
			Name:     filename,
		})
		if err != nil {
			slog.Error("Failed to insert character avatar into DB", "error", err)
			htmx.ServerError(w)
			return
		}

		if err := a.Storage.UploadAvatar(ctx, sess.UserID, assetID, avatar); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			a.discardAvatar(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}

		err = a.Queries.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
			ID:      characterID,
			OwnerID: sess.UserID,
			AssetID: &assetID,
		})
		if err != nil {
			slog.Error("Failed to link avatar to character", "error", err)
			a.discardAvatar(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}
	}

	htmx.Toast(w, "Updated avatar for "+character.Name)

	updated, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to reload character after avatar update", "error", err, "characterID", characterID.String())
		htmx.Redirect(w, "/characters")
		return
	}
	render(w, r, pages.Character(updated))
}

// UploadMonsterImage is UploadCharacterAvatar for the manual, and every part of
// its shape is that handler's for that handler's reasons: the row is written
// before the object because the row is the ledger for what lives in R2, a
// failure after the row is written is rolled back by a compensating delete, and
// a replacement is written to the SAME KEY so nothing is ever orphaned and
// nothing needs sweeping.
//
// It answers with the re-rendered card, which is the mutation case the fragment
// rules name -- a POST replying with the thing it just changed.
//
// The picture is an asset of type `monster` and not `token`. That member is for
// one-off images placed on a map that belong to no monster; this one is what the
// card, the editor bar and eventually the pawn are all drawn from.
func (a *App) UploadMonsterImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
		return
	}

	src, filename, ok := readImageUpload(w, r, "image", imageLimits)
	if !ok {
		return
	}
	encoded, err := images.EncodeWebP(images.Square(src, monsterImageSize))
	if err != nil {
		slog.Error("Failed to encode monster image as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	monster, err := a.Queries.GetMonsterAsset(ctx, queries.GetMonsterAssetParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "monster")
		return
	}
	if err != nil {
		slog.Error("Failed to get monster asset", "error", err)
		htmx.ServerError(w)
		return
	}

	if monster.AssetID != nil {
		// Replacing overwrites the existing key, so nothing can be orphaned and
		// the card's <img> src does not change -- which is why serveImage sends
		// no-cache rather than no-store, and why updated_at is bumped.
		assetID := *monster.AssetID
		if err := a.Storage.UploadMonsterImage(ctx, sess.UserID, assetID, encoded); err != nil {
			slog.Error("Failed to upload monster image", "error", err)
			htmx.ServerError(w)
			return
		}
		err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:       assetID,
			OwnerID:  sess.UserID,
			FileName: filename,
		})
		if err != nil {
			slog.Error("Failed to update monster image asset", "error", err)
			htmx.ServerError(w)
			return
		}
	} else {
		// The row is the ledger for what lives in R2, so it is written first
		// and rolled back if the upload never lands.
		assetID := ulid.Make()
		err := a.Queries.InsertMonsterImage(ctx, queries.InsertMonsterImageParams{
			ID:       assetID,
			OwnerID:  sess.UserID,
			FilePath: storage.MonsterImageKey(sess.UserID, assetID),
			FileName: filename,
			Name:     filename,
		})
		if err != nil {
			slog.Error("Failed to insert monster image into DB", "error", err)
			htmx.ServerError(w)
			return
		}

		if err := a.Storage.UploadMonsterImage(ctx, sess.UserID, assetID, encoded); err != nil {
			slog.Error("Failed to upload monster image", "error", err)
			a.discardMonsterImage(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}

		err = a.Queries.UpdateMonsterImage(ctx, queries.UpdateMonsterImageParams{
			ID:      monsterID,
			OwnerID: sess.UserID,
			AssetID: &assetID,
		})
		if err != nil {
			slog.Error("Failed to link image to monster", "error", err)
			a.discardMonsterImage(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}
	}

	htmx.Toast(w, "Updated image for "+monster.Name)

	updated, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      monsterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to reload monster after image update", "error", err, "monsterID", monsterID.String())
		htmx.Redirect(w, "/monsters")
		return
	}
	render(w, r, pages.MonsterCard(monsterSummary(updated)))
}

// UploadMap stores a map and queues it for tiling. It does not decode it, does
// not re-encode it and does not build its preview: all three moved to the
// tiling worker, which is the only thing in the app that ever holds a
// hundred-megapixel image in memory. What is left is a header check, a row, a
// PUT of the bytes that arrived, and queueing the work.
func (a *App) UploadMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	original, filename, contentType, ok := readImageBytes(w, r, "map", mapLimits)
	if !ok {
		return
	}

	assetID := ulid.Make()
	originalPath := storage.MapOriginalKey(sess.UserID, assetID)
	tileSize := sql.NullInt16{Int16: tiling.DefaultTileSize, Valid: true}

	// THE ROW IS WRITTEN FIRST AND QUEUED LAST, and the upload goes between
	// them. The row is the ledger for what lives in R2, so it has to exist
	// before the object does -- that is the rule everything else here obeys,
	// and it is why this is rolled back if the upload never lands. But a row
	// that is pending is a job, and a job's first act is to read the original
	// back out of the bucket. Writing 'pending' before the PUT returns leaves
	// a window the length of the upload in which the queue holds a job whose
	// input does not exist yet, and the worker looks every five seconds. It
	// loses that race often enough to see: the map fails to tile with a
	// missing key, and tiling the very same object succeeds the moment anyone
	// presses retry, because by then the PUT has finished.
	//
	// So the insert leaves tile_state NULL -- an owned row naming a key, which
	// no worker can claim -- and QueueMapForTiling below makes it work once
	// there is something to work on. Replacing a map already had this order.
	err := a.Queries.InsertMap(ctx, queries.InsertMapParams{
		ID:       assetID,
		OwnerID:  sess.UserID,
		FilePath: originalPath,
		FileName: filename,
		Name:     filename,
		TileSize: tileSize,
	})
	if err != nil {
		slog.Error("Failed to insert map", "error", err)
		htmx.ServerError(w)
		return
	}

	if err := a.Storage.UploadMapOriginal(ctx, sess.UserID, assetID, original, contentType); err != nil {
		slog.Error("Failed to upload map", "error", err)
		a.discardMap(ctx, sess.UserID, assetID)
		htmx.ServerError(w)
		return
	}

	// The same rollback as a failed upload: a row left un-queued would be a
	// card with an original behind it, no pyramid, and nothing that will ever
	// build one -- so the upload did not happen rather than half happened.
	err = a.Queries.QueueMapForTiling(ctx, queries.QueueMapForTilingParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to queue map for tiling", "error", err)
		a.discardMap(ctx, sess.UserID, assetID)
		htmx.ServerError(w)
		return
	}

	// The card is built from what was just written rather than by reading the
	// row back: the insert above is the whole of what this map is so far, and
	// a map with no pyramid has nothing else to show. It comes back pending,
	// so it starts polling the moment it lands on the page.
	htmx.Toast(w, filename+" uploaded.")
	render(w, r, pages.MapCard(pages.MapAsset{
		ID:       assetID.String(),
		Name:     filename,
		FileName: filename,
		State:    queries.AssetsTileStatePending,
	}))
}

func (a *App) DeleteMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}

	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "map")
		return
	}
	if err != nil {
		slog.Error("Failed to load map", "error", err)
		htmx.ServerError(w)
		return
	}

	// Objects first, row last: the row is the record that objects may exist,
	// so it goes only once R2 has confirmed they are gone. One prefix is the
	// whole map -- the original and every generation of its pyramid, including
	// one a worker is part way through writing.
	if err := a.Storage.DeletePrefix(ctx, storage.MapPrefix(sess.UserID, assetID)); err != nil {
		slog.Error("Failed to delete map objects", "error", err)
		htmx.ServerError(w)
		return
	}

	err = a.Queries.DeleteAsset(ctx, queries.DeleteAssetParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete asset row", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, m.Name+" deleted.")
}

func (a *App) ReplaceMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}

	// Ownership is checked before anything is written: the keys are built
	// from the session's user id, so writing first would land a stranger's
	// asset id in this user's namespace with no row behind it.
	_, err = a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "map")
		return
	}
	if err != nil {
		slog.Error("Failed to load map", "error", err)
		htmx.ServerError(w)
		return
	}

	original, filename, contentType, ok := readImageBytes(w, r, "map", mapLimits)
	if !ok {
		return
	}

	// THE ORIGINAL IS OVERWRITTEN AND NOTHING ELSE IS TOUCHED. Its key is
	// fixed, so replacing it orphans nothing; the tiles are the opposite --
	// they are never overwritten, only superseded. The pyramid that is serving
	// goes on serving under the generation tile_gen names, and the worker
	// deletes it only once it has a whole new one to put in its place. So
	// there is no window here in which the map has no tiles, and nothing to
	// clean up if this request fails halfway.
	if err := a.Storage.UploadMapOriginal(ctx, sess.UserID, assetID, original, contentType); err != nil {
		slog.Error("Failed to upload map", "error", err)
		htmx.ServerError(w)
		return
	}

	_, err = a.Queries.RequeueMapForTiling(ctx, queries.RequeueMapForTilingParams{
		ID:       assetID,
		OwnerID:  sess.UserID,
		FileName: filename,
	})
	if err != nil {
		slog.Error("Failed to requeue map for tiling", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, filename+" uploaded.")

	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to reload map after replace", "error", err, "assetID", assetID.String())
		htmx.Refresh(w)
		return
	}
	render(w, r, pages.MapCard(mapCard(m)))
}

// RetryMapTiling puts a map whose tiling gave up back in the queue. It is the
// button behind a card that failed, and it answers with that card, which is now
// a card that is waiting.
//
// IT IS A MUTATION AT A RESOURCE URL rather than a fragment route, and it
// returns the row it just changed -- the alternative is a POST that answers
// with nothing followed by a GET to fetch what it did.
func (a *App) RetryMapTiling(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}

	// The update is conditional on the row still being failed, and its result
	// is deliberately not read: a button pressed twice, or pressed on a card
	// that a background retry already picked up, changes nothing and is not an
	// error. What the owner gets back either way is the row as it now stands.
	_, err = a.Queries.RetryMapTiling(ctx, queries.RetryMapTilingParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to retry map tiling", "error", err)
		htmx.ServerError(w)
		return
	}

	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "map")
		return
	}
	if err != nil {
		slog.Error("Failed to load map after retry", "error", err, "assetID", assetID.String())
		htmx.ServerError(w)
		return
	}

	render(w, r, pages.MapCard(mapCard(m)))
}

func (a *App) ReplaceMapName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}

	name := strings.TrimSpace(r.FormValue("map-name"))
	if name == "" {
		name = "Untitled"
	}

	err = a.Queries.UpdateAssetName(ctx, queries.UpdateAssetNameParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to update map name", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" updated.")
}

// discardMap rolls back a map upload that failed after its row was written. The
// row is only dropped once R2 confirms the objects are gone, so a cleanup
// failure leaves the row behind as the record that they may still exist.
func (a *App) discardMap(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Storage.DeletePrefix(cleanupCtx, storage.MapPrefix(userID, assetID)); err != nil {
		slog.Error("Failed to clean up map objects; leaving the asset row behind", "error", err, "assetID", assetID.String())
		return
	}
	err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{
		ID:      assetID,
		OwnerID: userID,
	})
	if err != nil {
		slog.Error("Failed to delete asset row after cleaning up its objects", "error", err, "assetID", assetID.String())
	}
}

// discardMonsterImage rolls back a monster image upload that failed after its
// row was written, on the same terms as discardMap: the row is only dropped once
// R2 confirms the object is gone, so a cleanup failure leaves the row behind as
// the record that it may still exist.
func (a *App) discardMonsterImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Storage.Delete(cleanupCtx, storage.MonsterImageKey(userID, assetID)); err != nil {
		slog.Error("Failed to clean up monster image object; leaving the asset row behind", "error", err, "assetID", assetID.String())
		return
	}
	err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{
		ID:      assetID,
		OwnerID: userID,
	})
	if err != nil {
		slog.Error("Failed to delete asset row after cleaning up its object", "error", err, "assetID", assetID.String())
	}
}

// discardAvatar rolls back an avatar upload that failed after its row was
// written, on the same terms as discardMap.
func (a *App) discardAvatar(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Storage.Delete(cleanupCtx, storage.AvatarKey(userID, assetID)); err != nil {
		slog.Error("Failed to clean up avatar object; leaving the asset row behind", "error", err, "assetID", assetID.String())
		return
	}
	err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{
		ID:      assetID,
		OwnerID: userID,
	})
	if err != nil {
		slog.Error("Failed to delete asset row after cleaning up its object", "error", err, "assetID", assetID.String())
	}
}
