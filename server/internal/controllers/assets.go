package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"io"
	"log/slog"
	db "main/internal/database"
	"main/internal/helpers"
	"main/internal/queries"
	"main/internal/services"
	"main/internal/session"
	"main/templ/pages"
	"net/http"
	"strconv"

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
	maps, err := q.GetUserMaps(ctx, session.UserId)
	if err != nil {
		helpers.RedirectToError(w, r)
		return
	}

	pages.MapAssets(session, maps).Render(r.Context(), w)
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
	assetId, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	q := queries.New(db)
	result, err := q.GetImage(ctx, assetId)
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
	assetId, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	q := queries.New(db)
	result, err := q.GetImage(ctx, assetId)
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
	characterId, err := ulid.Parse(id)
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
	assetId := ulid.Make()
	q := queries.New(db)
	oldAsset, err := q.GetCharacterAssetByIDAndOwner(ctx, queries.GetCharacterAssetByIDAndOwnerParams{
		ID:      characterId,
		OwnerID: session.UserId,
	})
	if err != nil {
		slog.Error("Failed to get old character asset", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	if !oldAsset.AssetID.IsZero() {
		// NOTE: replacing overwrites the existing key, so nothing can be orphaned
		assetId = oldAsset.AssetID

		if err := services.UploadAvatar(ctx, session.UserId, assetId, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			helpers.HTMXServerError(w)
			return
		}

		if err := q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID:       assetId,
			OwnerID:  session.UserId,
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
			ID:       assetId,
			OwnerID:  session.UserId,
			FilePath: services.AvatarKey(session.UserId, assetId),
			FileName: filename,
			Name:     filename,
		})
		if err != nil {
			slog.Error("Failed to insert character avatar into DB", "error", err)
			helpers.HTMXServerError(w)
			return
		}

		if err := services.UploadAvatar(ctx, session.UserId, assetId, out.Bytes()); err != nil {
			slog.Error("Failed to upload character avatar", "error", err)
			discardAvatar(ctx, q, session.UserId, assetId)
			helpers.HTMXServerError(w)
			return
		}

		if err := q.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
			ID:      characterId,
			OwnerID: session.UserId,
			AssetID: assetId,
		}); err != nil {
			slog.Error("Failed to update character avatar", "error", err)
			discardAvatar(ctx, q, session.UserId, assetId)
			helpers.HTMXServerError(w)
			return
		}
	}

	helpers.HTMXToast(w, "Updated avatar for "+oldAsset.Name)

	characterRecord, err := q.GetCharacter(ctx, queries.GetCharacterParams{
		ID: characterId,
		OwnerID: session.UserId,
	})
	if err != nil {
		slog.Error("Failed to get character after update", "error", err, "assetId", assetId.String())
		helpers.HTMXRedirect(w, "/characters")
		return
	}

	character := queries.GetCharactersRow{
		AssetID: characterRecord.AssetID,
		Speed: characterRecord.Speed,
		ProficiencyBonus: characterRecord.ProficiencyBonus,
		CurrentHp: characterRecord.CurrentHp,
		MaxHp: characterRecord.MaxHp,
		Ac: characterRecord.Ac,
		Size: characterRecord.Size,
		Alignment: characterRecord.Alignment,
		ID: characterRecord.ID,
		Name: characterRecord.Name,
		Level: characterRecord.Level,
		Xp: characterRecord.Xp,
		Race: characterRecord.Race,
		Classes: characterRecord.Classes,
		Background: characterRecord.Background,
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

	assetId := ulid.Make()
	fullPath, previewPath := services.MapKeys(session.UserId, assetId)

	// NOTE: the row is the ledger for what lives in R2, so it is written first
	// and rolled back if the upload never lands
	q := queries.New(db)
	err = q.InsertMap(ctx, queries.InsertMapParams{
		ID:          assetId,
		OwnerID:     session.UserId,
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

	if err := services.UploadMap(ctx, session.UserId, assetId, fullOut.Bytes(), resizedOut.Bytes()); err != nil {
		slog.Error("Failed to upload map", "error", err)
		discardMap(ctx, q, session.UserId, assetId)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, filename+" uploaded.")
	newMap := queries.GetUserMapsRow{
		ID:          assetId,
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
	assetId, err := ulid.Parse(id)
	if err != nil {
		slog.Error("Failed to parse asset ID", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	result, err := q.GetUserImage(ctx, queries.GetUserImageParams{
		ID:      assetId,
		OwnerID: session.UserId,
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
		ID:      assetId,
		OwnerID: session.UserId,
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
	assetId, err := ulid.Parse(id)
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
	err = services.UploadMap(ctx, session.UserId, assetId, fullOut.Bytes(), resizedOut.Bytes())
	if err != nil {
		slog.Error("Failed to upload map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	err = q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
		ID:       assetId,
		OwnerID:  session.UserId,
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
	assetId, err := ulid.Parse(id)
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
		ID:      assetId,
		OwnerID: session.UserId,
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
func discardMap(ctx context.Context, q *queries.Queries, userId ulid.ULID, assetId ulid.ULID) {
	cleanupCtx, cancel := services.CleanupContext(ctx)
	defer cancel()

	if err := services.DeleteMapObjects(cleanupCtx, userId, assetId); err != nil {
		slog.Error("Failed to clean up map objects; leaving the asset row behind", "error", err, "assetId", assetId.String())
		return
	}
	if err := q.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetId,
		OwnerID: userId,
	}); err != nil {
		slog.Error("Failed to delete asset row after cleaning up its objects", "error", err, "assetId", assetId.String())
	}
}

// discardAvatar rolls back an avatar upload that failed after its row was
// written, on the same terms as discardMap.
func discardAvatar(ctx context.Context, q *queries.Queries, userId ulid.ULID, assetId ulid.ULID) {
	cleanupCtx, cancel := services.CleanupContext(ctx)
	defer cancel()

	if err := services.DeleteImage(cleanupCtx, services.AvatarKey(userId, assetId)); err != nil {
		slog.Error("Failed to clean up avatar object; leaving the asset row behind", "error", err, "assetId", assetId.String())
		return
	}
	if err := q.DeleteUsersAsset(cleanupCtx, queries.DeleteUsersAssetParams{
		ID:      assetId,
		OwnerID: userId,
	}); err != nil {
		slog.Error("Failed to delete asset row after cleaning up its object", "error", err, "assetId", assetId.String())
	}
}
