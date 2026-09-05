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
	db "tabletopper/internal/database"
	"tabletopper/internal/helpers"
	"tabletopper/internal/queries"
	"tabletopper/internal/services"
	"tabletopper/internal/session"
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

func MapAssetsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	q := queries.New(db)
	maps, err := q.GetUserMaps(ctx, session.UserID)
	if err != nil {
		helpers.RedirectToError(w, r)
		return
	}

	pages.MapAssets(maps).Render(r.Context(), w)
}

func AssetsPage(w http.ResponseWriter, r *http.Request) {
	helpers.Redirect(w, r, "/assets/maps")
}

func GetImagePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()

	id := r.PathValue("id")
	if id == "" {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	q := queries.New(db)
	result, err := q.GetImage(ctx, assetID)
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	file_path := result.FilePath
	if result.PreviewPath.Valid {
		file_path = result.PreviewPath.String
	}

	data, err := services.GetImage(ctx, file_path)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()

	id := r.PathValue("id")
	if id == "" {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	q := queries.New(db)
	result, err := q.GetImage(ctx, assetID)
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	data, err := services.GetImage(ctx, result.FilePath)
	if err != nil {
		slog.Error("Failed to get image from R2", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func UploadCharacterAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		helpers.HTMXServerError(w)
		return
	}
	characterID, err := ulid.Parse(id)
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		slog.Error("Failed to get avatar from form", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		helpers.HTMXCustomError(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	resized := imaging.Fill(src, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)
	var out bytes.Buffer
	if err := webp.Encode(&out, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode image as webp", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	filename := header.Filename
	assetID := ulid.Make()
	q := queries.New(db)
	oldAsset, err := q.GetCharacterAssetByIDAndOwner(ctx, queries.GetCharacterAssetByIDAndOwnerParams{
		ID:      characterID,
		OwnerID: session.UserID,
	})
	if err != nil {
		slog.Error("Failed to get old character asset", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	if oldAsset.AssetID != nil {
		// NOTE: replacing overwrites the existing key, so nothing can be orphaned
		assetID = *oldAsset.AssetID

		if err := services.UploadAvatar(ctx, session.UserID, assetID, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			helpers.HTMXServerError(w)
			return
		}

		if err := q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:       assetID,
			OwnerID:  session.UserID,
			FileName: filename,
		}); err != nil {
			slog.Error("Failed to update avatar asset", "error", err)
			helpers.HTMXServerError(w)
			return
		}
	} else {
		// NOTE: the row is the ledger for what lives in R2, so it is written
		// first and rolled back if the upload never lands
		err = q.InsertAvatar(ctx, queries.InsertAvatarParams{
			ID:       assetID,
			OwnerID:  session.UserID,
			FilePath: services.AvatarKey(session.UserID, assetID),
			FileName: filename,
			Name:     filename,
		})
		if err != nil {
			slog.Error("Failed to insert character avatar into DB", "error", err)
			helpers.HTMXServerError(w)
			return
		}

		if err := services.UploadAvatar(ctx, session.UserID, assetID, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			discardAvatar(ctx, q, session.UserID, assetID)
			helpers.HTMXServerError(w)
			return
		}

		if err := q.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
			ID:      characterID,
			OwnerID: session.UserID,
			AssetID: &assetID,
		}); err != nil {
			slog.Error("Failed to update character avatar", "error", err)
			discardAvatar(ctx, q, session.UserID, assetID)
			helpers.HTMXServerError(w)
			return
		}
	}

	helpers.HTMXToast(w, "Updated avatar for "+oldAsset.Name)

	characterRecord, err := q.GetCharacter(ctx, queries.GetCharacterParams{
		ID:      characterID,
		OwnerID: session.UserID,
	})
	if err != nil {
		slog.Error("Failed to get character after update", "error", err, "assetID", assetID.String())
		helpers.HTMXRedirect(w, "/characters")
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
	pages.Character(character).Render(r.Context(), w)
}

func UploadMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	file, header, err := r.FormFile("map")
	if err != nil {
		slog.Error("Failed to get map from form", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		helpers.HTMXCustomError(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	filename := header.Filename

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	resized := imaging.Fill(src, mapPreviewSize, mapPreviewSize, imaging.Center, imaging.Lanczos)
	var resizedOut bytes.Buffer
	if err := webp.Encode(&resizedOut, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode preview image as webp", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	var fullOut bytes.Buffer
	if err := webp.Encode(&fullOut, src, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode full image as webp", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	assetID := ulid.Make()
	fullPath, previewPath := services.MapKeys(session.UserID, assetID)

	// NOTE: the row is the ledger for what lives in R2, so it is written first
	// and rolled back if the upload never lands
	q := queries.New(db)
	err = q.InsertMap(ctx, queries.InsertMapParams{
		ID:          assetID,
		OwnerID:     session.UserID,
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		FileName:    filename,
		Name:        filename,
	})
	if err != nil {
		slog.Error("Failed to insert map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	if err := services.UploadMap(ctx, session.UserID, assetID, fullOut.Bytes(), resizedOut.Bytes()); err != nil {
		slog.Error("Failed to upload map", "error", err)
		discardMap(ctx, q, session.UserID, assetID)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, filename+" uploaded.")
	newMap := queries.GetUserMapsRow{
		ID:          assetID,
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		FileName:    filename,
		Name:        filename,
	}
	pages.MapCard(newMap).Render(r.Context(), w)
}

func DeleteMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		helpers.HTMXServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	result, err := q.GetUserImage(ctx, queries.GetUserImageParams{
		ID:      assetID,
		OwnerID: session.UserID,
	})
	if err != nil {
		slog.Error("Failed to query user's image", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	err = services.DeleteImage(ctx, result.FilePath)
	if err != nil {
		slog.Error("Failed to delete image", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	if result.PreviewPath.Valid {
		err = services.DeleteImage(ctx, result.PreviewPath.String)
		if err != nil {
			slog.Error("Failed to delete preview image", "error", err)
			helpers.HTMXServerError(w)
			return
		}
	}

	err = q.DeleteUsersAsset(ctx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: session.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete asset from DB", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, result.Name+" deleted.")
}

func ReplaceMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		helpers.HTMXServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	file, header, err := r.FormFile("map")
	if err != nil {
		slog.Error("Failed to get map from form", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		helpers.HTMXCustomError(w, "Unsupported Image Type", "Only PNG, JPEG, and WEBP images are allowed. Refresh the page and try again.", http.StatusUnsupportedMediaType)
		return
	}

	filename := header.Filename

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	resized := imaging.Fill(src, mapPreviewSize, mapPreviewSize, imaging.Center, imaging.Lanczos)
	var resizedOut bytes.Buffer
	if err := webp.Encode(&resizedOut, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode preview image as webp", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	var fullOut bytes.Buffer
	if err := webp.Encode(&fullOut, src, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode full image as webp", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	// NOTE: replacing overwrites the existing keys, so nothing can be orphaned
	err = services.UploadMap(ctx, session.UserID, assetID, fullOut.Bytes(), resizedOut.Bytes())
	if err != nil {
		slog.Error("Failed to upload map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	err = q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
		ID:       assetID,
		OwnerID:  session.UserID,
		FileName: filename,
	})
	if err != nil {
		slog.Error("Failed to update map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, filename+" uploaded.")
	helpers.HTMXRefresh(w)
}

func ReplaceMapName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := db.Get()
	session := session.FromContext(ctx)

	id := r.PathValue("id")
	if id == "" {
		helpers.HTMXServerError(w)
		return
	}
	assetID, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	newName := r.FormValue("map-name")
	if len(newName) == 0 {
		newName = "Untitled"
	}

	q := queries.New(db)
	err = q.UpdateAssetName(ctx, queries.UpdateAssetNameParams{
		ID:      assetID,
		OwnerID: session.UserID,
		Name:    newName,
	})
	if err != nil {
		slog.Error("Failed to update map name", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, newName+" updated.")
}

// discardMap rolls back a map upload that failed after its row was written. The
// row is only dropped once R2 confirms the objects are gone, so a cleanup
// failure leaves the row behind as the record that they may still exist.
func discardMap(ctx context.Context, q *queries.Queries, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := services.CleanupContext(ctx)
	defer cancel()

	if err := services.DeleteMapObjects(cleanupCtx, userID, assetID); err != nil {
		slog.Error("Failed to clean up map objects; leaving the asset row behind", "error", err, "assetID", assetID.String())
		return
	}
	if err := q.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: userID,
	}); err != nil {
		slog.Error("Failed to delete asset row after cleaning up its objects", "error", err, "assetID", assetID.String())
	}
}

// discardAvatar rolls back an avatar upload that failed after its row was
// written, on the same terms as discardMap.
func discardAvatar(ctx context.Context, q *queries.Queries, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := services.CleanupContext(ctx)
	defer cancel()

	if err := services.DeleteImage(cleanupCtx, services.AvatarKey(userID, assetID)); err != nil {
		slog.Error("Failed to clean up avatar object; leaving the asset row behind", "error", err, "assetID", assetID.String())
		return
	}
	if err := q.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetID,
		OwnerID: userID,
	}); err != nil {
		slog.Error("Failed to delete asset row after cleaning up its object", "error", err, "assetID", assetID.String())
	}
}
