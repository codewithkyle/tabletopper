package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	maxUploadBytes      = 8 << 20
	maxUploadPixels     = 40_000_000
	maxMapBytes         = 128 << 20
	maxMapPixels        = 150_000_000
	multipartMemory     = 32 << 20
	uploadReadDeadline  = 10 * time.Minute
	uploadWriteDeadline = uploadReadDeadline + 30*time.Second
	avatarSize          = 96
	monsterImageSize    = 256
)

type uploadLimits struct {
	bytes  int64
	pixels int64
}

var (
	imageLimits = uploadLimits{bytes: maxUploadBytes, pixels: maxUploadPixels}
	mapLimits   = uploadLimits{bytes: maxMapBytes, pixels: maxMapPixels}
)

func extendUploadDeadlines(w http.ResponseWriter) {
	now := time.Now()
	controller := http.NewResponseController(w)
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
func willTileAgain(state queries.AssetsTileState, attempts uint8) bool {
	return state == queries.AssetsTileStateFailed && int(attempts) < tiling.MaxAttempts
}
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
func (a *App) GetImage(w http.ResponseWriter, r *http.Request) {
	a.serveImage(w, r, false)
}
func (a *App) GetImagePreview(w http.ResponseWriter, r *http.Request) {
	a.serveImage(w, r, true)
}
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
func (a *App) streamImage(w http.ResponseWriter, r *http.Request, key string, etag string) {
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body, size, err := a.Storage.Get(r.Context(), key)
	if err != nil {
		if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
			slog.Debug("Image fetch abandoned by the client", "key", key)
		} else {
			slog.Error("Failed to get image from R2", "error", err, "key", key)
		}
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
		slog.Debug("Image stream ended early", "error", err, "key", key)
	}
}
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

type uploadProblem struct {
	Heading string
	Message string
	Status  int
}

func (p *uploadProblem) alert(w http.ResponseWriter) {
	htmx.Error(w, p.Heading, p.Message, p.Status)
}

var (
	errUnreadableUpload = &uploadProblem{
		Heading: "Upload Failed",
		Message: "The upload could not be read. Refresh the page and try again.",
		Status:  http.StatusBadRequest,
	}
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

var decodeSlots = make(chan struct{}, 2)

const decodeWait = 10 * time.Second

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
func serverBusy(w http.ResponseWriter) {
	htmx.Error(w, "Server Busy", "Too many uploads are being processed. Try again in a moment.", http.StatusServiceUnavailable)
}
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
		sess.ProfileImageURL = session.AvatarURL(&assetID, sess.ProfileImageURL)
	}
	htmx.Toast(w, "Updated your profile picture")
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
	if character.AssetID != nil && character.FilePath.Valid {
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
func (a *App) UploadMonsterImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	monsterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "monster")
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterImageControl(pages.MonsterImage{
		MonsterID: monsterID.String(),
		Name:      monster.Name,
		ImageID:   assetID.String(),
	}))
}
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
func (a *App) UploadMap(w http.ResponseWriter, r *http.Request) {
	assetID, filename, ok := a.storeMap(w, r)
	if !ok {
		return
	}
	render(w, r, pages.MapCard(pages.MapAsset{
		ID:       assetID.String(),
		Name:     filename,
		FileName: filename,
		State:    queries.AssetsTileStatePending,
	}))
}
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
	err := a.Queries.InsertMap(ctx, queries.InsertMapParams{
		ID:        assetID,
		OwnerID:   sess.UserID,
		FilePath:  originalPath,
		FileName:  filename,
		Name:      filename,
		TileSize:  tileSize,
		SizeBytes: header.Size,
	})
	if err != nil {
		slog.Error("Failed to insert map", "error", err)
		htmx.ServerError(w)
		return ulid.ULID{}, "", false
	}
	if err := a.Storage.UploadMapOriginal(ctx, sess.UserID, assetID, file, header.Size, contentType); err != nil {
		slog.Error("Failed to upload map", "error", err)
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.DeletePrefix(c, storage.MapPrefix(sess.UserID, assetID))
		})
		htmx.ServerError(w)
		return ulid.ULID{}, "", false
	}
	err = a.Queries.QueueMapForTiling(ctx, queries.QueueMapForTilingParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to queue map for tiling", "error", err)
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
	file, header, contentType, ok := openImageUpload(w, r, "map", mapLimits)
	if !ok {
		return
	}
	defer file.Close()
	filename := header.Filename
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
func (a *App) requeueMap(w http.ResponseWriter, ctx context.Context, ownerID ulid.ULID, assetID ulid.ULID) (queries.Asset, bool) {
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
func assetName(name string) string {
	runes := []rune(name)
	if len(runes) > pages.AssetNameLimit {
		return string(runes[:pages.AssetNameLimit])
	}
	return name
}
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
func (a *App) discardAssetRow(ctx context.Context, userID, assetID ulid.ULID) {
	a.discardAsset(ctx, userID, assetID, func(context.Context) error { return nil })
}
