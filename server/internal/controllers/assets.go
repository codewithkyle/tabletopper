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

	cards, err := a.mapList(ctx, sess.UserID, "")
	if err != nil {
		slog.Error("Failed to load maps", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.MapAssets(cards))
}

// mapList is the maps shelf in either of its two states -- everything the owner
// has, or what matched a search -- so the page and the search fragment build the
// same cards from the same function. See libraryList, which is the same shape
// for the two kinds that are one stored image.
func (a *App) mapList(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MapAsset, error) {
	var rows []queries.Asset
	var err error

	if term == "" {
		rows, err = a.Queries.GetMaps(ctx, ownerID)
	} else {
		rows, err = a.Queries.SearchMaps(ctx, queries.SearchMapsParams{
			OwnerID: ownerID,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}

	cards := make([]pages.MapAsset, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, mapCard(row))
	}

	return cards, nil
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
		ID:        m.ID.String(),
		Name:      m.Name,
		FileName:  m.FileName,
		State:     m.TileState.AssetsTileState,
		AutoRetry: willTileAgain(m.TileState.AssetsTileState, m.TileAttempts),
	}
	if m.TileGen != nil {
		card.Generation = m.TileGen.String()
	}

	return card
}

// willTileAgain answers whether the tiling worker is coming back to this row on
// its own, which is the difference between the two things a failed card can say.
//
// IT IS RequeueFailedTilingJobs' CONDITION, MINUS THE TIMESTAMP. That statement
// takes every failed map with fewer than tiling.MaxAttempts attempts whose lease
// went cold, so the count is what decides whether a retry is coming and the
// timestamp only decides when. A card cannot usefully say "in eleven minutes" --
// the sweep is on an interval and the row is one of a batch -- so it says a few
// minutes and is right about the part that matters.
func willTileAgain(state queries.AssetsTileState, attempts uint8) bool {
	return state == queries.AssetsTileStateFailed && int(attempts) < tiling.MaxAttempts
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

	// A MAP'S ORIGINAL IS NEVER SERVED, and this is the branch that holds that
	// invariant rather than the storage layout that documents it. For every
	// other kind file_path is a WebP this app encoded; for a map it is the PNG
	// or JPEG that arrived, up to 128 MiB, and answering with it would stream
	// all of that through this process under a Content-Type of image/webp.
	//
	// So a map has exactly one picture here -- the preview, and only once the
	// tiler has built one. Both URLs 404 until then, which is what the card
	// markup already assumes: MapAsset.Usable() gates the <img>, so nothing
	// links either of them in the meantime.
	key := asset.FilePath
	switch {
	case asset.Type == queries.AssetsTypeMap:
		if !preview || !asset.PreviewPath.Valid {
			http.NotFound(w, r)
			return
		}
		key = asset.PreviewPath.String
	case preview && asset.PreviewPath.Valid:
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
		// A CANCELLED REQUEST IS THE BROWSER AND NOT THE STORE, and on the
		// room page it is routine rather than rare: the renderer drops every
		// tile fetch that leaves the viewport, so a pan abandons a handful and
		// a zoom that changes pyramid level abandons everything in flight. An
		// <img> does the same the moment the page navigates. Logged at error
		// level those bury the failures that are real -- a missing object, a
		// bad credential -- under a line per abandoned tile.
		if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
			slog.Debug("Image fetch abandoned by the client", "key", key)
		} else {
			slog.Error("Failed to get image from R2", "error", err, "key", key)
		}

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
// THE HEADER COMES BACK WHOLE rather than just the filename off it, because the
// map path needs the declared size as well: it hands the file straight to R2
// and a PUT wants a ContentLength. Both fields are the multipart part's own,
// which is what makes them free to return.
func openImageUpload(w http.ResponseWriter, r *http.Request, field string, limits uploadLimits) (multipart.File, *multipart.FileHeader, string, bool) {
	if problem := parseUploadForm(w, r, limits); problem != nil {
		problem.alert(w)
		return nil, nil, "", false
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		slog.Error("Failed to get upload from form", "field", field, "error", err)
		htmx.Error(w, "Upload Failed", "No image was attached. Refresh the page and try again.", http.StatusBadRequest)
		return nil, nil, "", false
	}

	contentType, problem := inspectImage(file, field, limits)
	if problem != nil {
		file.Close()
		problem.alert(w)
		return nil, nil, "", false
	}

	return file, header, contentType, true
}

// uploadProblem is something wrong with an upload, kept as data rather than
// written straight to the response, because the same fault has to be told two
// different ways. A route that exists to take one picture raises the alert
// dialog; a form where the picture is one field among several puts the sentence
// in its own error block and leaves everything else the user typed alone.
//
// The heading and the status are the alert's half. The message is the half both
// use, which is why it is a sentence and not a phrase.
type uploadProblem struct {
	Heading string
	Message string
	Status  int
}

func (p *uploadProblem) alert(w http.ResponseWriter) {
	htmx.Error(w, p.Heading, p.Message, p.Status)
}

var (
	// errUnreadableUpload is a body that arrived broken or a file that would
	// not seek. There is nothing for the user to fix and nothing useful to
	// tell them beyond "try again".
	errUnreadableUpload = &uploadProblem{
		Heading: "Upload Failed",
		Message: "The upload could not be read. Refresh the page and try again.",
		Status:  http.StatusBadRequest,
	}

	// errNotMultipart is a body that carried no file part at all. It is a
	// separate value from the one above BECAUSE A CALLER CAN GO ON WITHOUT
	// ONE: a form whose picture is optional has still had its text fields
	// read by the time this comes back, so it is told apart by identity
	// rather than by its message, which is the same one.
	errNotMultipart = &uploadProblem{
		Heading: "Upload Failed",
		Message: "The upload could not be read. Refresh the page and try again.",
		Status:  http.StatusBadRequest,
	}

	errUnsupportedImage = &uploadProblem{
		Heading: "Unsupported Image Type",
		Message: "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.",
		Status:  http.StatusUnsupportedMediaType,
	}
)

// parseUploadForm reads a multipart body under one kind of upload's limits: the
// deadlines lifted, the byte cap enforced by the reader rather than by trusting
// a header, and the form parsed.
//
// IT IS SEPARATE FROM opening the file because a form can carry an optional
// picture beside other fields, and that caller needs the form parsed whether or
// not there is a file in it. ParseMultipartForm reads the ordinary fields on
// its way through, so by the time it reports that there was no multipart body
// at all, a urlencoded post's values are already in r.PostForm.
func parseUploadForm(w http.ResponseWriter, r *http.Request, limits uploadLimits) *uploadProblem {
	extendUploadDeadlines(w)

	r.Body = http.MaxBytesReader(w, r.Body, limits.bytes)
	err := r.ParseMultipartForm(multipartMemory)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, http.ErrNotMultipart):
		return errNotMultipart
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &uploadProblem{
			Heading: "Upload Too Large",
			Message: fmt.Sprintf("Images must be %d MiB or smaller.", limits.bytes>>20),
			Status:  http.StatusRequestEntityTooLarge,
		}
	}

	slog.Error("Failed to parse multipart form", "error", err)

	return errUnreadableUpload
}

// inspectImage is the header pass, on a file that is already open. It leaves
// the file rewound to the start, so whatever the caller does next reads the
// whole thing.
func inspectImage(file multipart.File, field string, limits uploadLimits) (string, *uploadProblem) {
	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		slog.Warn("Failed to read upload header", "field", field, "error", err)
		return "", errUnsupportedImage
	}
	switch format {
	case "png", "jpeg", "webp":
	default:
		return "", errUnsupportedImage
	}
	// int64, so the multiplication cannot wrap on a declared canvas large
	// enough to try -- the whole point of this check is a header nobody sane
	// wrote.
	if int64(cfg.Width)*int64(cfg.Height) > limits.pixels {
		return "", &uploadProblem{
			Heading: "Image Too Large",
			Message: fmt.Sprintf("Images must be %d megapixels or fewer.", limits.pixels/1_000_000),
			Status:  http.StatusRequestEntityTooLarge,
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		slog.Error("Failed to rewind upload after reading its header", "field", field, "error", err)
		return "", errUnreadableUpload
	}

	return "image/" + format, nil
}

// openOptionalImageUpload is openImageUpload for a form where the picture is
// one field among several and leaving it empty is allowed. The form must
// already have been parsed by parseUploadForm.
//
// A file input that was left alone still sends a part -- browsers send one with
// an empty filename and no bytes -- so an empty part is read as no picture
// rather than as a picture that will not decode. Nothing is written to the
// response here: the caller reports the problem the way its own page needs it.
func openOptionalImageUpload(r *http.Request, field string, limits uploadLimits) (multipart.File, string, *uploadProblem) {
	if r.MultipartForm == nil {
		return nil, "", nil
	}
	headers := r.MultipartForm.File[field]
	if len(headers) == 0 || headers[0].Size == 0 {
		return nil, "", nil
	}

	file, _, err := r.FormFile(field)
	if err != nil {
		slog.Error("Failed to get upload from form", "field", field, "error", err)
		return nil, "", errUnreadableUpload
	}

	if _, problem := inspectImage(file, field, limits); problem != nil {
		file.Close()
		return nil, "", problem
	}

	return file, headers[0].Filename, nil
}

// decodeSlots is how many uploads may be decoded at once, process-wide.
//
// THE DECODE IS THE ONE STEP THAT MULTIPLIES, which is why it is the step that
// is bounded rather than the request. The header pass has already refused
// anything over the pixel cap, but the cap is generous: forty megapixels of
// NRGBA is about 160 MB, imaging may copy it once more to apply an orientation
// tag, and the few megabytes of PNG that ask for all of that arrive over a
// connection that costs nothing to open. Ten at once is a gigabyte and a half
// of heap for ten thumbnails. The resize and the encode that follow read that
// buffer and produce something a few kilobytes long, so bounding the decode
// bounds the process.
//
// TWO, and the reason is the tiling worker: it holds a whole map's pixels while
// it works and was deliberately serialised for exactly this -- see the tiling
// package comment. This is the request path's share of the same budget, and it
// is a ceiling of roughly two full-size images rather than a throughput
// target, because the decode itself takes tens of milliseconds and a queue of
// two is not a queue anyone waits in.
//
// The decoded image outlives its slot: the caller still has to resize and
// encode it. That is accepted -- the peak is the decode, and what is bounded
// is how many peaks coincide.
var decodeSlots = make(chan struct{}, 2)

// decodeWait is how long an upload waits for a slot before the request is
// answered 503. Long enough that a burst of a handful clears, short enough
// that a reader is told rather than left holding a spinner.
const decodeWait = 10 * time.Second

// decodeUpload decodes one upload inside the bound above, and reports
// context.DeadlineExceeded when no slot came free in time -- which the caller
// has to tell apart from a file it could not read, because one is the server's
// fault and the other is the upload's.
func decodeUpload(ctx context.Context, file io.Reader) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, decodeWait)
	defer cancel()

	select {
	case decodeSlots <- struct{}{}:
		defer func() { <-decodeSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return imaging.Decode(file, imaging.AutoOrientation(true))
}

// serverBusy is what a request that could not get a decode slot is answered
// with. 503 rather than 500: nothing is broken and trying again works.
func serverBusy(w http.ResponseWriter) {
	htmx.Error(w, "Server Busy", "Too many uploads are being processed. Try again in a moment.", http.StatusServiceUnavailable)
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
//
// IT GOES THROUGH decodeUpload AND NOT STRAIGHT TO imaging, so every path that
// decodes in a request shares one bound. The other one is newMonsterPicture,
// which cannot use this function because it answers into a dialog rather than
// through the alert header.
func readImageUpload(w http.ResponseWriter, r *http.Request, field string, limits uploadLimits) (image.Image, string, bool) {
	file, header, _, ok := openImageUpload(w, r, field, limits)
	if !ok {
		return nil, "", false
	}
	defer file.Close()

	src, err := decodeUpload(r.Context(), file)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Gave up waiting for a decode slot", "field", field)
		serverBusy(w)
		return nil, "", false
	}
	if err != nil {
		slog.Warn("Failed to decode upload", "field", field, "error", err)
		unsupportedImage(w)
		return nil, "", false
	}

	return src, header.Filename, true
}

func unsupportedImage(w http.ResponseWriter) {
	errUnsupportedImage.alert(w)
}

// UploadAccountAvatar is the picture beside the welcome on the homepage, which
// is the one picture in this app that a person had no way to choose until now.
//
// IT OVERRIDES CLERK RATHER THAN OVERWRITING IT. users.profile_image_url goes on
// holding whatever Clerk said at the last login and goes on being refreshed by
// the next one; this writes a second column that session.AvatarURL prefers. So
// somebody who signed up through Discord and would rather not be their Discord
// avatar at the table gets to say so, without the app having to reach into an
// identity provider it does not own.
//
// IT IS THE SAME SHAPE AS UploadCharacterAvatar FOR THAT HANDLER'S REASONS: the
// row is the ledger for what lives in R2 so it is written before the object, a
// failure after the row is written is rolled back by a compensating delete, and
// a replacement goes to the SAME KEY off the stored file_path -- so nothing is
// ever orphaned, nothing needs sweeping, and the second upload cannot land
// beside the first while the row goes on naming the first.
//
// THERE IS NO OWNERSHIP LOOKUP TO DO FIRST, which is the one way it differs.
// The character handler reads the character before it touches the body, because
// the id in the path could name anybody's; the only account this can write is
// the one that is signed in, so the session IS the check and there is nothing
// to look up before the decode.
//
// It answers with the re-rendered avatar and not the whole badge, which is the
// mutation case the fragment rules name -- a POST replying with the thing it
// just changed. The badge is deliberately not the target: it carries the
// homepage's fade-in, and swapping it would replay that animation on every
// upload.
func (a *App) UploadAccountAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	src, filename, ok := readImageUpload(w, r, "avatar", imageLimits)
	if !ok {
		return
	}

	picture, err := images.EncodeWebP(images.Square(src, avatarSize))
	if err != nil {
		slog.Error("Failed to encode account avatar as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	current, err := a.Queries.GetUserAvatar(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to read the account's avatar", "error", err)
		htmx.ServerError(w)
		return
	}

	// THE PATH DECIDES AND NOT THE POINTER, which is GetUserAvatar's own note:
	// the column has no foreign key behind it, so it can name an asset that is
	// gone, and the join hands that back as an id with no file_path. Branching
	// on the id alone would write the replacement to an empty key and report
	// success. Falling through to the insert mints a new asset and relinks the
	// account, so uploading again is what repairs it.
	if current.AvatarAssetID != nil && current.FilePath.Valid {
		if err := a.Storage.UploadImage(ctx, current.FilePath.String, picture); err != nil {
			slog.Error("Failed to upload account avatar", "error", err)
			htmx.ServerError(w)
			return
		}

		err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:        *current.AvatarAssetID,
			OwnerID:   sess.UserID,
			FileName:  filename,
			SizeBytes: int64(len(picture)),
		})
		if err != nil {
			slog.Error("Failed to update account avatar asset", "error", err)
			htmx.ServerError(w)
			return
		}
	} else {
		assetID := ulid.Make()
		err := a.Queries.InsertProfilePicture(ctx, queries.InsertProfilePictureParams{
			ID:        assetID,
			OwnerID:   sess.UserID,
			FilePath:  storage.AvatarKey(sess.UserID, assetID),
			FileName:  assetName(filename),
			Name:      assetName(filename),
			SizeBytes: int64(len(picture)),
		})
		if err != nil {
			slog.Error("Failed to insert account avatar into DB", "error", err)
			htmx.ServerError(w)
			return
		}

		discard := func(c context.Context) error {
			return a.Storage.Delete(c, storage.AvatarKey(sess.UserID, assetID))
		}

		if err := a.Storage.UploadImage(ctx, storage.AvatarKey(sess.UserID, assetID), picture); err != nil {
			slog.Error("Failed to upload account avatar", "error", err)
			a.discardAsset(ctx, sess.UserID, assetID, discard)
			htmx.ServerError(w)
			return
		}

		err = a.Queries.SetUserAvatar(ctx, queries.SetUserAvatarParams{
			ID:            sess.UserID,
			AvatarAssetID: &assetID,
		})
		if err != nil {
			slog.Error("Failed to link avatar to account", "error", err)
			a.discardAsset(ctx, sess.UserID, assetID, discard)
			htmx.ServerError(w)
			return
		}

		// The session was built before this upload existed, so its resolved
		// picture is still Clerk's. A REPLACEMENT needs no such line: the
		// asset id did not change, so neither did the URL.
		sess.ProfileImageURL = session.AvatarURL(&assetID, sess.ProfileImageURL)
	}

	htmx.Toast(w, "Updated your profile picture")

	// THE URL IS THE SAME ONE ON A REPLACEMENT, which is what serveImage's
	// no-cache ETag is for: the browser still asks, the row's updated_at has
	// moved, and the new bytes come back at the address the old ones had.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AccountAvatar(sess))
}

func (a *App) UploadCharacterAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}

	// THE LOOKUP IS THE OWNERSHIP CHECK AND IT GOES FIRST, which is the order
	// UploadJournalImage already used and said why. GetCharacterAsset is scoped
	// to the session's owner, so a request naming somebody else's character
	// misses here -- before the multipart body is touched, before a decoder has
	// allocated anything. The other way round, any signed-in user could spend
	// 160 MB of this process's heap on a forty-megapixel PNG and be answered
	// 404 for their trouble. The body sits unread on the connection while the
	// statement runs, which costs nothing.
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

	// THE ROW HAS TO BE THERE, NOT JUST THE POINTER. characters.asset_id has no
	// foreign key behind it -- nothing in this schema does -- so a character can
	// name an assets row that has been deleted, and the LEFT JOIN in
	// GetCharacterAsset hands that back as an id with no file_path. Branching on
	// the id alone would send the replacement to an empty key and answer 500 on
	// a character that is simply missing its picture. Falling through to the
	// insert instead mints a new asset and relinks the character, so uploading a
	// portrait is what fixes it.
	if character.AssetID != nil && character.FilePath.Valid {
		// NOTE: replacing overwrites the existing key, so nothing can be orphaned
		//
		// THE KEY COMES OFF THE ROW. A portrait written before the asset library
		// existed is stored under users/{owner}/avatars/{id}, because that is
		// what `avatar` meant then; 20260907120000 moved the row's type and
		// moved nothing in the bucket. Rebuilding the key here would send the
		// replacement to users/{owner}/portraits/{id}, leaving the object the
		// row still points at untouched -- so the upload would appear to
		// succeed and the portrait would never change.
		assetID := *character.AssetID
		if err := a.Storage.UploadImage(ctx, character.FilePath.String, avatar); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			htmx.ServerError(w)
			return
		}
		err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:        assetID,
			OwnerID:   sess.UserID,
			FileName:  filename,
			SizeBytes: int64(len(avatar)),
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
		err := a.Queries.InsertCharacterPortrait(ctx, queries.InsertCharacterPortraitParams{
			ID:        assetID,
			OwnerID:   sess.UserID,
			FilePath:  storage.CharacterPortraitKey(sess.UserID, assetID),
			FileName:  assetName(filename),
			Name:      assetName(filename),
			SizeBytes: int64(len(avatar)),
		})
		if err != nil {
			slog.Error("Failed to insert character avatar into DB", "error", err)
			htmx.ServerError(w)
			return
		}

		if err := a.Storage.UploadCharacterPortrait(ctx, sess.UserID, assetID, avatar); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
				return a.Storage.Delete(c, storage.CharacterPortraitKey(sess.UserID, assetID))
			})
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
			a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
				return a.Storage.Delete(c, storage.CharacterPortraitKey(sess.UserID, assetID))
			})
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

	// Owner-scoped lookup first, for the reason UploadCharacterAvatar does it
	// first: it is the ownership check, and a stranger's monster id must not
	// cost a decode before it is answered 404.
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

	var assetID ulid.ULID
	if monster.AssetID != nil {
		// Replacing overwrites the existing key, so nothing can be orphaned and
		// the card's <img> src does not change -- which is why serveImage sends
		// no-cache rather than no-store, and why updated_at is bumped.
		assetID = *monster.AssetID
		if err := a.Storage.UploadMonsterImage(ctx, sess.UserID, assetID, encoded); err != nil {
			slog.Error("Failed to upload monster image", "error", err)
			htmx.ServerError(w)
			return
		}
		err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:        assetID,
			OwnerID:   sess.UserID,
			FileName:  filename,
			SizeBytes: int64(len(encoded)),
		})
		if err != nil {
			slog.Error("Failed to update monster image asset", "error", err)
			htmx.ServerError(w)
			return
		}
	} else {
		assetID, err = a.attachMonsterImage(ctx, sess.UserID, monsterID, encoded, filename)
		if err != nil {
			slog.Error("Failed to attach monster image", "error", err)
			htmx.ServerError(w)
			return
		}
	}

	htmx.Toast(w, "Updated image for "+monster.Name)

	// The reply is the control itself rather than the card around it, because
	// a picture can be set from either page and this is the one piece both of
	// them have. Nothing else on the card changes when an image does.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterImageControl(pages.MonsterImage{
		MonsterID: monsterID.String(),
		Name:      monster.Name,
		ImageID:   assetID.String(),
	}))
}

// attachMonsterImage gives a monster its first picture: an assets row, the
// object, and the column on the monster that points at it. It is one function
// because the create dialog can do this as well as the upload route, and the
// order is the property being protected rather than a detail of either caller.
//
// THE ROW IS THE LEDGER FOR WHAT LIVES IN R2, so it is written before the
// object and names the key the object will land at. An object written first and
// a row that never followed is a file nothing remembers: no page can render it,
// no delete will find it, and the sweeper works from these same rows.
//
// EVERY FAILURE AFTER THE ROW ROLLS IT BACK, which is what keeps that ledger
// honest in the other direction -- a row pointing at an object that never
// landed would render a broken picture on the card forever.
//
// The asset id comes back because the caller draws the picture it just stored.
func (a *App) attachMonsterImage(ctx context.Context, ownerID, monsterID ulid.ULID, encoded []byte, filename string) (ulid.ULID, error) {
	assetID := ulid.Make()

	err := a.Queries.InsertMonsterImage(ctx, queries.InsertMonsterImageParams{
		ID:        assetID,
		OwnerID:   ownerID,
		FilePath:  storage.MonsterImageKey(ownerID, assetID),
		FileName:  filename,
		Name:      filename,
		SizeBytes: int64(len(encoded)),
	})
	if err != nil {
		return ulid.ULID{}, fmt.Errorf("asset row: %w", err)
	}

	if err := a.Storage.UploadMonsterImage(ctx, ownerID, assetID, encoded); err != nil {
		a.discardAsset(ctx, ownerID, assetID, func(c context.Context) error {
			return a.Storage.Delete(c, storage.MonsterImageKey(ownerID, assetID))
		})
		return ulid.ULID{}, fmt.Errorf("object: %w", err)
	}

	// The link runs last -- after the object is in the bucket -- so a monster
	// never names an asset whose upload failed.
	err = a.Queries.UpdateMonsterImage(ctx, queries.UpdateMonsterImageParams{
		ID:      monsterID,
		OwnerID: ownerID,
		AssetID: &assetID,
	})
	if err != nil {
		a.discardAsset(ctx, ownerID, assetID, func(c context.Context) error {
			return a.Storage.Delete(c, storage.MonsterImageKey(ownerID, assetID))
		})
		return ulid.ULID{}, fmt.Errorf("link: %w", err)
	}

	return assetID, nil
}

// UploadMap stores a map and queues it for tiling. It does not decode it, does
// not re-encode it and does not build its preview: all three moved to the
// tiling worker, which is the only thing in the app that ever holds a
// hundred-megapixel image in memory. What is left is a header check, a row, a
// PUT of the bytes that arrived, and queueing the work.
//
// AND IT DOES NOT HOLD THE BYTES EITHER. The file goes to R2 as the reader
// ParseMultipartForm handed over -- a temp file for anything past 32 MiB -- so
// a 128 MiB map costs this process the multipart spill and nothing on top of
// it. Reading it into a slice first, which is what this used to do, meant two
// concurrent uploads were a quarter of a gigabyte of heap holding bytes that
// were already on disk.
func (a *App) UploadMap(w http.ResponseWriter, r *http.Request) {
	assetID, filename, ok := a.storeMap(w, r)
	if !ok {
		return
	}

	// The card is built from what was just written rather than by reading the
	// row back: the insert above is the whole of what this map is so far, and
	// a map with no pyramid has nothing else to show. It comes back pending,
	// so it starts polling the moment it lands on the page.
	render(w, r, pages.MapCard(pages.MapAsset{
		ID:       assetID.String(),
		Name:     filename,
		FileName: filename,
		State:    queries.AssetsTileStatePending,
	}))
}

// storeMap is the upload itself, without the card at the end of it.
//
// IT IS SPLIT OUT BECAUSE THE MAP PICKER UPLOADS TOO, and the picker's card is
// not this one: it is a button that puts the map on a layer, where this one is a
// name box, a Replace and a Delete. The work is identical and the markup is not,
// so the work is here and each caller renders its own. A false return has already
// answered the request.
func (a *App) storeMap(w http.ResponseWriter, r *http.Request) (ulid.ULID, string, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	file, header, contentType, ok := openImageUpload(w, r, "map", mapLimits)
	if !ok {
		return ulid.ULID{}, "", false
	}
	defer file.Close()

	filename := header.Filename
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
		// The original's length, which is the part header's. The tiles the
		// worker builds from it are derived and are not counted -- see
		// InsertMap in assets.sql.
		SizeBytes: header.Size,
	})
	if err != nil {
		slog.Error("Failed to insert map", "error", err)
		htmx.ServerError(w)
		return ulid.ULID{}, "", false
	}

	if err := a.Storage.UploadMapOriginal(ctx, sess.UserID, assetID, file, header.Size, contentType); err != nil {
		slog.Error("Failed to upload map", "error", err)
		// A prefix and not a key: a map owns a directory, because it is a tile
		// pyramid rather than one file.
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.DeletePrefix(c, storage.MapPrefix(sess.UserID, assetID))
		})
		htmx.ServerError(w)
		return ulid.ULID{}, "", false
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
		// A prefix and not a key: a map owns a directory, because it is a tile
		// pyramid rather than one file.
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.DeletePrefix(c, storage.MapPrefix(sess.UserID, assetID))
		})
		htmx.ServerError(w)
		return ulid.ULID{}, "", false
	}

	htmx.Toast(w, filename+" uploaded.")

	return assetID, filename, true
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

	// Opened after the ownership check above, and streamed rather than read --
	// see UploadMap for both.
	file, header, contentType, ok := openImageUpload(w, r, "map", mapLimits)
	if !ok {
		return
	}
	defer file.Close()

	filename := header.Filename

	// THE ORIGINAL IS OVERWRITTEN AND NOTHING ELSE IS TOUCHED. Its key is
	// fixed, so replacing it orphans nothing; the tiles are the opposite --
	// they are never overwritten, only superseded. The pyramid that is serving
	// goes on serving under the generation tile_gen names, and the worker
	// deletes it only once it has a whole new one to put in its place. So
	// there is no window here in which the map has no tiles, and nothing to
	// clean up if this request fails halfway.
	if err := a.Storage.UploadMapOriginal(ctx, sess.UserID, assetID, file, header.Size, contentType); err != nil {
		slog.Error("Failed to upload map", "error", err)
		htmx.ServerError(w)
		return
	}

	_, err = a.Queries.RequeueMapForTiling(ctx, queries.RequeueMapForTilingParams{
		SizeBytes: header.Size,
		ID:        assetID,
		OwnerID:   sess.UserID,
		FileName:  filename,
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

	m, ok := a.requeueMap(w, r.Context(), sess.UserID, assetID)
	if !ok {
		return
	}

	render(w, r, pages.MapCard(mapCard(m)))
}

// requeueMap is the retry itself, without the card at the end of it, split out
// for the reason storeMap is: the map picker offers the same button and answers
// with its own markup. A false return has already answered the request.
func (a *App) requeueMap(w http.ResponseWriter, ctx context.Context, ownerID ulid.ULID, assetID ulid.ULID) (queries.Asset, bool) {
	// The update is conditional on the row still being failed, and its result
	// is deliberately not read: a button pressed twice, or pressed on a card
	// that a background retry already picked up, changes nothing and is not an
	// error. What the owner gets back either way is the row as it now stands.
	_, err := a.Queries.RetryMapTiling(ctx, queries.RetryMapTilingParams{
		ID:      assetID,
		OwnerID: ownerID,
	})
	if err != nil {
		slog.Error("Failed to retry map tiling", "error", err)
		htmx.ServerError(w)
		return queries.Asset{}, false
	}

	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{
		ID:      assetID,
		OwnerID: ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "map")
		return queries.Asset{}, false
	}
	if err != nil {
		slog.Error("Failed to load map after retry", "error", err, "assetID", assetID.String())
		htmx.ServerError(w)
		return queries.Asset{}, false
	}

	return m, true
}

func (a *App) ReplaceMapName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}

	name := assetName(strings.TrimSpace(r.FormValue("map-name")))
	if name == "" {
		name = "Untitled"
	}

	err = a.Queries.UpdateAssetName(ctx, queries.UpdateAssetNameParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Type:    queries.AssetsTypeMap,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to update map name", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" updated.")
}

// assetName cuts a name to what assets.name holds. MySQL runs strict, so a
// longer value is a driver error rather than a truncation -- a 500 on a save
// that a person would read as the name simply not sticking.
//
// IT CUTS RATHER THAN REFUSES, and that is a statement about who can reach it.
// Every box that posts a name carries pages.AssetNameLimit as maxlength, so a
// browser cannot send an over-long one; what arrives here too long was composed
// by something else, and there is no one to show an error to. A filename is the
// other caller and is not typed at all.
//
// The count is in runes because VARCHAR counts characters, which is also what
// maxlength counts -- so the two agree on a name of accented characters, where
// a byte count would refuse one the column would have taken.
func assetName(name string) string {
	runes := []rune(name)
	if len(runes) > pages.AssetNameLimit {
		return string(runes[:pages.AssetNameLimit])
	}

	return name
}

// discardAsset rolls back an upload that failed after its row was written, and
// it is the one rollback for all six kinds -- a map, a portrait, a monster's
// picture, a journal image, a library token or avatar, and a track.
//
// THE ORDER IS THE POINT AND IT IS THE SAME ORDER EVERY DELETE IN THIS APP
// USES. The object goes first and the row only once R2 has confirmed it, so a
// cleanup that fails leaves the row behind as the record that the object may
// still be there -- something a later delete or a sweep can find. The other way
// round leaves an orphan under a key nothing in the database names, which
// nothing will ever collect.
//
// IT DETACHES FROM THE REQUEST. storage.CleanupContext is what makes that true,
// and it matters because the likeliest reason to be here at all is the reader
// having closed the tab: cleaning up under a cancelled context would fail
// immediately and leave exactly the half-made thing this exists to remove.
//
// remove IS A CLOSURE BECAUSE THAT IS THE WHOLE OF WHAT DIFFERED between the
// six functions this replaced. Five delete one key and one deletes a prefix --
// a map owns a directory rather than a file, because it is a tile pyramid --
// and the key itself is built five different ways. Everything around it was the
// same two statements and the same two log lines, copied.
//
// Nothing is returned. Every caller is already answering a failure of its own,
// and there is nothing a reader could do about a rollback that did not work.
func (a *App) discardAsset(ctx context.Context, userID, assetID ulid.ULID, remove func(context.Context) error) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := remove(cleanupCtx); err != nil {
		slog.Error("Failed to clean up an asset's objects; leaving the row behind", "error", err, "assetID", assetID.String())
		return
	}

	err := a.Queries.DeleteAsset(cleanupCtx, queries.DeleteAssetParams{
		ID:      assetID,
		OwnerID: userID,
	})
	if err != nil {
		slog.Error("Failed to delete an asset row after cleaning up its objects", "error", err, "assetID", assetID.String())
	}
}

// discardAssetRow drops a row that never got as far as owning an object, which
// today is a track whose presigned URL was never handed out. It is
// discardAsset with a remove that has nothing to do, written out rather than
// passed as a nil check so the absence is deliberate at the call site.
func (a *App) discardAssetRow(ctx context.Context, userID, assetID ulid.ULID) {
	a.discardAsset(ctx, userID, assetID, func(context.Context) error { return nil })
}
