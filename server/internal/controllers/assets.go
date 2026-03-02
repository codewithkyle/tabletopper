package controllers

import (
	"bytes"
	"context"
	"image"
	"io"
	"log/slog"
	db "main/internal/database"
	"main/internal/queries"
	"main/internal/services"
	"main/internal/session"
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

func GetImage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	assetId, err := ulid.Parse(id)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	q := queries.New(db)
	result, err := q.GetImage(ctx, assetId[:])
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	data, err := services.GetImage(ctx, result.FilePath)
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
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
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/characters", http.StatusTemporaryRedirect)
		return
	}
	characterId, err := ulid.Parse(id)
	if err != nil {
		http.Redirect(w, r, "/characters", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		slog.Error("Failed to get avatar from form", "error", err)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	default:
		http.Error(w, "unsupported image type", http.StatusUnsupportedMediaType)
		return
	}

	limited := io.LimitReader(file, maxUploadBytes)
	src, _, err := image.Decode(limited)
	if err != nil {
		slog.Error("Failed to decode image", "error", err)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	resized := imaging.Fill(src, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)
	var out bytes.Buffer
	if err := webp.Encode(&out, resized, &webp.Options{Quality: float32(outQuality)}); err != nil {
		slog.Error("Failed to encode image as webp", "error", err)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	//filename := header.Filename
	assetId := ulid.Make()
	filepath, err := services.UploadAvatar(ctx, session.UserId, assetId, out.Bytes())
	if err != nil {
		slog.Error("Failed to upload character avatar", "error", err)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	q := queries.New(db)
	err = q.InsertAvatar(ctx, queries.InsertAvatarParams{
		ID: assetId[:],
		OwnerID: session.UserId[:],
		FilePath: filepath,
	})
	if err != nil {
		slog.Error("Failed to insert character avatar into DB", "error", err, "file", filepath)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	err = q.UpdateCharacterAvatar(ctx, queries.UpdateCharacterAvatarParams{
		ID: characterId[:],
		OwnerID: session.UserId[:],
		AssetID: assetId[:],
	})
	if err != nil {
		slog.Error("Failed to update character avatar", "error", err, "file", filepath)
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
