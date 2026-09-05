package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	// Uploads are decoded with image.Decode, which only knows the formats
	// that registered themselves. PNG and JPEG are registered here rather
	// than relied on from a dependency's imports; webp registers itself in
	// the chai2010 package below.
	_ "image/jpeg"
	_ "image/png"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/templ/pages"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/oklog/ulid/v2"
)

const (
	maxUploadBytes = 8 << 20 // 8 MiB
	avatarSize     = 96
	mapPreviewSize = 256
	outQuality     = 75
)

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

	render(w, r, pages.MapAssets(maps))
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

// serveImage streams the object rather than buffering it, and answers a
// conditional request from the row alone. The ETag is the asset id plus
// updated_at, which every write to an asset bumps, so a browser that has the
// current bytes gets a 304 without R2 being asked. Cache-Control is no-cache,
// not no-store: the browser keeps the bytes, it just has to ask first, which
// is what lets a replaced avatar show up on the next paint at the same URL.
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

	etag := fmt.Sprintf(`"%s-%d"`, assetID, asset.UpdatedAt.Unix())
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	body, size, err := a.Storage.Get(ctx, key)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err, "key", key)
		http.NotFound(w, r)
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

// readImageUpload pulls one image out of a multipart form. It writes the
// response itself when something is wrong with the upload, so a caller only
// has to stop: the false return means "already answered".
//
// The format comes from what actually decoded, not from the Content-Type the
// browser claimed. imaging registers GIF, BMP and TIFF decoders as a side
// effect of importing it, so decoding alone is not the allowlist.
func readImageUpload(w http.ResponseWriter, r *http.Request, field string) (image.Image, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			htmx.Error(w, "Upload Too Large", "Images must be 8 MiB or smaller.", http.StatusRequestEntityTooLarge)
			return nil, "", false
		}
		slog.Error("Failed to parse multipart form", "error", err)
		htmx.Error(w, "Upload Failed", "The upload could not be read. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", false
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		slog.Error("Failed to get upload from form", "field", field, "error", err)
		htmx.Error(w, "Upload Failed", "No image was attached. Refresh the page and try again.", http.StatusBadRequest)
		return nil, "", false
	}
	defer file.Close()

	src, format, err := image.Decode(file)
	if err != nil {
		slog.Warn("Failed to decode upload", "field", field, "error", err)
		unsupportedImage(w)
		return nil, "", false
	}
	switch format {
	case "png", "jpeg", "webp":
	default:
		unsupportedImage(w)
		return nil, "", false
	}

	return src, header.Filename, true
}

func unsupportedImage(w http.ResponseWriter) {
	htmx.Error(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
}

// square crops and scales img to a size-by-size square, centred.
func square(img image.Image, size int) image.Image {
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}

func encodeWebP(img image.Image) ([]byte, error) {
	var out bytes.Buffer
	if err := webp.Encode(&out, img, &webp.Options{Quality: outQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (a *App) UploadCharacterAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "character")
		return
	}

	src, filename, ok := readImageUpload(w, r, "avatar")
	if !ok {
		return
	}
	avatar, err := encodeWebP(square(src, avatarSize))
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

func (a *App) UploadMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	src, filename, ok := readImageUpload(w, r, "map")
	if !ok {
		return
	}
	preview, err := encodeWebP(square(src, mapPreviewSize))
	if err != nil {
		slog.Error("Failed to encode map preview as webp", "error", err)
		htmx.ServerError(w)
		return
	}
	full, err := encodeWebP(src)
	if err != nil {
		slog.Error("Failed to encode map as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	assetID := ulid.Make()
	fullPath, previewPath := storage.MapKeys(sess.UserID, assetID)

	// NOTE: the row is the ledger for what lives in R2, so it is written first
	// and rolled back if the upload never lands
	err = a.Queries.InsertMap(ctx, queries.InsertMapParams{
		ID:          assetID,
		OwnerID:     sess.UserID,
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		FileName:    filename,
		Name:        filename,
	})
	if err != nil {
		slog.Error("Failed to insert map", "error", err)
		htmx.ServerError(w)
		return
	}

	if err := a.Storage.UploadMap(ctx, sess.UserID, assetID, full, preview); err != nil {
		slog.Error("Failed to upload map", "error", err)
		a.discardMap(ctx, sess.UserID, assetID)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, filename+" uploaded.")
	now := time.Now()
	render(w, r, pages.MapCard(queries.Asset{
		ID:          assetID,
		OwnerID:     sess.UserID,
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		Type:        queries.AssetsTypeMap,
		FileName:    filename,
		Name:        filename,
		CreatedAt:   now,
		UpdatedAt:   now,
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
	// so it goes only once R2 has confirmed they are gone.
	if err := a.Storage.DeleteMapObjects(ctx, sess.UserID, assetID); err != nil {
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

	src, filename, ok := readImageUpload(w, r, "map")
	if !ok {
		return
	}
	preview, err := encodeWebP(square(src, mapPreviewSize))
	if err != nil {
		slog.Error("Failed to encode map preview as webp", "error", err)
		htmx.ServerError(w)
		return
	}
	full, err := encodeWebP(src)
	if err != nil {
		slog.Error("Failed to encode map as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	// NOTE: replacing overwrites the existing keys, so nothing can be orphaned
	if err := a.Storage.UploadMap(ctx, sess.UserID, assetID, full, preview); err != nil {
		slog.Error("Failed to upload map", "error", err)
		htmx.ServerError(w)
		return
	}

	err = a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
		ID:       assetID,
		OwnerID:  sess.UserID,
		FileName: filename,
	})
	if err != nil {
		slog.Error("Failed to update map", "error", err)
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
	render(w, r, pages.MapCard(m))
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

	if err := a.Storage.DeleteMapObjects(cleanupCtx, userID, assetID); err != nil {
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
