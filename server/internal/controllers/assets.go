package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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

func (a *App) MapAssetsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	maps, err := a.Queries.GetUserMaps(ctx, sess.UserID)
	if err != nil {
		redirectToError(w, r)
		return
	}

	render(w, r, pages.MapAssets(maps))
}

func (a *App) AssetsPage(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/assets/maps")
}

func (a *App) GetImagePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFound(w, r)
		return
	}

	result, err := a.Queries.GetImage(ctx, assetID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file_path := result.FilePath
	if result.PreviewPath.Valid {
		file_path = result.PreviewPath.String
	}

	data, err := a.Storage.Get(ctx, file_path)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) GetImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFound(w, r)
		return
	}

	result, err := a.Queries.GetImage(ctx, assetID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data, err := a.Storage.Get(ctx, result.FilePath)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) UploadCharacterAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		htmx.ServerError(w)
		return
	}
	characterID, err := ulid.Parse(id)
	if err != nil {
		htmx.ServerError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		htmx.ServerError(w)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		slog.Error("Failed to get avatar from form", "error", err)
		htmx.ServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		htmx.Error(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		htmx.ServerError(w)
		return
	}
	resized := imaging.Fill(src, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)
	var out bytes.Buffer
	if err := webp.Encode(&out, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode image as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	filename := header.Filename
	assetID := ulid.Make()
	oldAsset, err := a.Queries.GetCharacterAssetByIDAndOwner(ctx, queries.GetCharacterAssetByIDAndOwnerParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to get old character asset", "error", err)
		htmx.ServerError(w)
		return
	}

	if oldAsset.AssetID != nil {
		// NOTE: replacing overwrites the existing key, so nothing can be orphaned
		assetID = *oldAsset.AssetID

		if err := a.Storage.UploadAvatar(ctx, sess.UserID, assetID, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			htmx.ServerError(w)
			return
		}

		if err := a.Queries.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:       assetID,
			OwnerID:  sess.UserID,
			FileName: filename,
		}); err != nil {
			slog.Error("Failed to update avatar asset", "error", err)
			htmx.ServerError(w)
			return
		}
	} else {
		// NOTE: the row is the ledger for what lives in R2, so it is written
		// first and rolled back if the upload never lands
		err = a.Queries.InsertAvatar(ctx, queries.InsertAvatarParams{
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

		if err := a.Storage.UploadAvatar(ctx, sess.UserID, assetID, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			a.discardAvatar(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}

		if err := a.Queries.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
			ID:      characterID,
			OwnerID: sess.UserID,
			AssetID: &assetID,
		}); err != nil {
			slog.Error("Failed to update character avatar", "error", err)
			a.discardAvatar(ctx, sess.UserID, assetID)
			htmx.ServerError(w)
			return
		}
	}

	htmx.Toast(w, "Updated avatar for "+oldAsset.Name)

	characterRecord, err := a.Queries.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      characterID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to get character after update", "error", err, "assetID", assetID.String())
		htmx.Redirect(w, "/characters")
		return
	}

	character := queries.GetCharactersRow{
		AssetID:          characterRecord.AssetID,
		Speed:            characterRecord.Speed,
		ProficiencyBonus: characterRecord.ProficiencyBonus,
		CurrentHP:        characterRecord.CurrentHP,
		MaxHP:            characterRecord.MaxHP,
		AC:               characterRecord.AC,
		Size:             characterRecord.Size,
		Alignment:        characterRecord.Alignment,
		ID:               characterRecord.ID,
		Name:             characterRecord.Name,
		Level:            characterRecord.Level,
		XP:               characterRecord.XP,
		Race:             characterRecord.Race,
		Classes:          characterRecord.Classes,
		Background:       characterRecord.Background,
	}
	render(w, r, pages.Character(character))
}

func (a *App) UploadMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		htmx.ServerError(w)
		return
	}

	file, header, err := r.FormFile("map")
	if err != nil {
		slog.Error("Failed to get map from form", "error", err)
		htmx.ServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		htmx.Error(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	filename := header.Filename

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		htmx.ServerError(w)
		return
	}
	resized := imaging.Fill(src, mapPreviewSize, mapPreviewSize, imaging.Center, imaging.Lanczos)
	var resizedOut bytes.Buffer
	if err := webp.Encode(&resizedOut, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode preview image as webp", "error", err)
		htmx.ServerError(w)
		return
	}
	var fullOut bytes.Buffer
	if err := webp.Encode(&fullOut, src, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode full image as webp", "error", err)
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

	if err := a.Storage.UploadMap(ctx, sess.UserID, assetID, fullOut.Bytes(), resizedOut.Bytes()); err != nil {
		slog.Error("Failed to upload map", "error", err)
		a.discardMap(ctx, sess.UserID, assetID)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, filename+" uploaded.")
	newMap := queries.GetUserMapsRow{
		ID:          assetID,
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		FileName:    filename,
		Name:        filename,
	}
	render(w, r, pages.MapCard(newMap))
}

func (a *App) DeleteMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		htmx.ServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		htmx.ServerError(w)
		return
	}

	result, err := a.Queries.GetUserImage(ctx, queries.GetUserImageParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to query user's image", "error", err)
		htmx.ServerError(w)
		return
	}

	err = a.Storage.Delete(ctx, result.FilePath)
	if err != nil {
		slog.Error("Failed to delete image", "error", err)
		htmx.ServerError(w)
		return
	}

	if result.PreviewPath.Valid {
		err = a.Storage.Delete(ctx, result.PreviewPath.String)
		if err != nil {
			slog.Error("Failed to delete preview image", "error", err)
			htmx.ServerError(w)
			return
		}
	}

	err = a.Queries.DeleteUsersAsset(ctx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete asset from DB", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, result.Name+" deleted.")
}

func (a *App) ReplaceMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		htmx.ServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		htmx.ServerError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		htmx.ServerError(w)
		return
	}

	file, header, err := r.FormFile("map")
	if err != nil {
		slog.Error("Failed to get map from form", "error", err)
		htmx.ServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		htmx.Error(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	filename := header.Filename

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		htmx.ServerError(w)
		return
	}
	resized := imaging.Fill(src, mapPreviewSize, mapPreviewSize, imaging.Center, imaging.Lanczos)
	var resizedOut bytes.Buffer
	if err := webp.Encode(&resizedOut, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode preview image as webp", "error", err)
		htmx.ServerError(w)
		return
	}
	var fullOut bytes.Buffer
	if err := webp.Encode(&fullOut, src, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode full image as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	// NOTE: replacing overwrites the existing keys, so nothing can be orphaned
	err = a.Storage.UploadMap(ctx, sess.UserID, assetID, fullOut.Bytes(), resizedOut.Bytes())
	if err != nil {
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
	htmx.Refresh(w)
}

func (a *App) ReplaceMapName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		htmx.ServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		htmx.ServerError(w)
		return
	}

	newName := r.FormValue("map-name")
	if len(newName) == 0 {
		newName = "Untitled"
	}

	err = a.Queries.UpdateAssetName(ctx, queries.UpdateAssetNameParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Name:    newName,
	})
	if err != nil {
		slog.Error("Failed to update map name", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, newName+" updated.")
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
	if err := a.Queries.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: userID,
	}); err != nil {
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
	if err := a.Queries.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: userID,
	}); err != nil {
		slog.Error("Failed to delete asset row after cleaning up its object", "error", err, "assetID", assetID.String())
	}
}
