package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
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
	outQuality     = 75
)

func MapAssetsPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		helpers.RedirectToError(w,r)
		return
	}

	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		helpers.RedirectToSignIn(w, r)
		return
	}

	pages.MapAssets(session).Render(r.Context(), w)
}

func AssetsPage(w http.ResponseWriter, r *http.Request) {
	helpers.Redirect(w,r,"/assets/maps")
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
	filepath, err := services.UploadAvatar(ctx, session.UserId, assetId, out.Bytes())
	if err != nil {
		slog.Error("Failed to upload character avatar", "error", err)
		helpers.HTMXServerError(w)
		return
	}

	q := queries.New(db)
	oldAsset, err := q.GetCharacterAssetByIDAndOwner(ctx, queries.GetCharacterAssetByIDAndOwnerParams{
		ID:      characterId[:],
		OwnerID: session.UserId[:],
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to get old character asset", "error", err)
			helpers.HTMXServerError(w)
			return
		}
	} else {
		err = services.DeleteImage(ctx, oldAsset.FilePath)
		if err != nil {
			slog.Error("Failed to delete old asset from R2", "error", err, "file_path", oldAsset.FilePath)
			helpers.HTMXServerError(w)
			return
		}
	}

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

	helpers.HTMXToast(w, "Updated avatar for "+oldAsset.Name)
	helpers.HTMXRefresh(w)
}
