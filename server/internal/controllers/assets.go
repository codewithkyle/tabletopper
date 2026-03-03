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
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.RedirectToError(w, r)
		return
	}

	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.RedirectToSignIn(w, r)
		return
	}

	q := queries.New(db)
	maps, err := q.GetUserMaps(ctx, session.UserId[:])
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
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	_, err = session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

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
	result, err := q.GetImage(ctx, assetId[:])
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	file_path := result.FilePath
	if !result.PreviewPath.Valid {
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
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	_, err = session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

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
	result, err := q.GetImage(ctx, assetId[:])
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
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.RedirectToSignIn(w, r)
		return
	}

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
		ID:      characterId[:],
		OwnerID: session.UserId[:],
	})
	if err != nil {
		slog.Error("Failed to get old character asset", "error", err)
		helpers.HTMXServerError(w)
		return
	}
	if len(oldAsset.AssetID) > 0 {
		assetId = ulid.ULID(oldAsset.AssetID)
	}

	filepath, err := services.UploadAvatar(ctx, session.UserId, assetId, out.Bytes())
	if err != nil {
		slog.Error("Failed to upload character avatar", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	if len(oldAsset.AssetID) == 0 {
		err = q.InsertAvatar(ctx, queries.InsertAvatarParams{
			ID:       assetId[:],
			OwnerID:  session.UserId[:],
			FilePath: filepath,
			FileName: filename,
			Name:     filename,
		})
		if err != nil {
			slog.Error("Failed to insert character avatar into DB", "error", err, "file", filepath)
			helpers.HTMXServerError(w)
			return
		}

		err = q.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
			ID:      characterId[:],
			OwnerID: session.UserId[:],
			AssetID: assetId[:],
		})
		if err != nil {
			slog.Error("Failed to update character avatar", "error", err, "file", filepath)
			helpers.HTMXServerError(w)
			return
		}
	} else {
		err := q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
			ID: assetId[:],
			OwnerID: session.UserId[:],
			FileName: filename,
		})
		if err != nil {
			slog.Error("Failed to update avatar asset", "error", err, "file", filepath)
			helpers.HTMXServerError(w)
			return
		}
	}

	helpers.HTMXToast(w, "Updated avatar for "+oldAsset.Name)
	helpers.HTMXRefresh(w)
}

func UploadMap(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.HTMXRedirect(w, "/sign-in")
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

	assetId := ulid.Make()
	fullPath, previewPath, err := services.UploadMap(ctx, session.UserId, assetId, resizedOut.Bytes(), fullOut.Bytes())
	if err != nil {
		slog.Error("Failed to upload map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	err = q.InsertMap(ctx, queries.InsertMapParams{
		ID:          assetId[:],
		OwnerID:     session.UserId[:],
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

	helpers.HTMXToast(w, filename+" uploaded.")
	newMap := queries.GetUserMapsRow{
		ID:          assetId[:],
		FilePath:    fullPath,
		PreviewPath: sql.NullString{Valid: true, String: previewPath},
		FileName:    filename,
		Name:        filename,
	}
	pages.MapCard(newMap).Render(r.Context(), w)
}

func DeleteMap(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}

	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}

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
		ID:      assetId[:],
		OwnerID: session.UserId[:],
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
		ID:      assetId[:],
		OwnerID: session.UserId[:],
	})
	if err != nil {
		slog.Error("Failed to delete asset from DB", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, result.Name+" deleted.")
}

func ReplaceMap(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.HTMXRedirect(w, "/sign-in")
		return
	}

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

	_, _, err = services.UploadMap(ctx, session.UserId, assetId, resizedOut.Bytes(), fullOut.Bytes())
	if err != nil {
		slog.Error("Failed to upload map", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	err = q.UpdateAssetFileName(ctx, queries.UpdateAssetFileNameParams{
		ID:       assetId[:],
		OwnerID:  session.UserId[:],
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
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.HTMXServerError(w)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.HTMXRedirect(w, "/sign-in")
		return
	}

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
		ID:      assetId[:],
		OwnerID: session.UserId[:],
		Name:    newName,
	})
	if err != nil {
		slog.Error("Failed to update map name", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	helpers.HTMXToast(w, newName+" updated.")
}
