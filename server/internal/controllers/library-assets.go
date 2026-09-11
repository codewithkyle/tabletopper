package controllers

import (
	"context"
	"database/sql"
	"errors"
	"image"
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/templ/pages"

	"github.com/a-h/templ"
	"github.com/oklog/ulid/v2"
)























const (
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	avatarLibrarySize = 256

	
	
	
	
	
	tokenSize = 512
)











type assetKind struct {
	
	
	
	Type queries.AssetsType

	
	
	Slug string

	
	
	One string
}




type libraryKind struct {
	assetKind

	
	Key func(userID ulid.ULID, assetID ulid.ULID) string

	
	
	
	
	
	Store func(src image.Image) image.Image

	
	
	
	
	
	Page  func(assets []pages.LibraryAsset) templ.Component
	Cards func(assets []pages.LibraryAsset, query string) templ.Component
}

var (
	avatarKind = libraryKind{
		assetKind: assetKind{Type: queries.AssetsTypeAvatar, Slug: "avatars", One: "avatar"},
		Key:       storage.AvatarKey,
		Store:     func(src image.Image) image.Image { return images.Square(src, avatarLibrarySize) },
		Page:      pages.AvatarAssets,
		Cards:     pages.AvatarCards,
	}

	tokenKind = libraryKind{
		assetKind: assetKind{Type: queries.AssetsTypeToken, Slug: "tokens", One: "token"},
		Key:       storage.TokenKey,
		Store:     func(src image.Image) image.Image { return images.Fit(src, tokenSize) },
		Page:      pages.TokenAssets,
		Cards:     pages.TokenCards,
	}

	
	
	musicKind = assetKind{Type: queries.AssetsTypeMusic, Slug: "music", One: "track"}
)




func (a *App) AvatarAssetsPage(w http.ResponseWriter, r *http.Request) {
	a.libraryPage(w, r, avatarKind)
}
func (a *App) UploadAvatar(w http.ResponseWriter, r *http.Request) { a.uploadLibrary(w, r, avatarKind) }
func (a *App) ReplaceAvatar(w http.ResponseWriter, r *http.Request) {
	a.replaceLibrary(w, r, avatarKind)
}
func (a *App) RenameAvatar(w http.ResponseWriter, r *http.Request) {
	a.renameLibrary(w, r, avatarKind.assetKind)
}
func (a *App) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	a.deleteLibrary(w, r, avatarKind.assetKind)
}

func (a *App) TokenAssetsPage(w http.ResponseWriter, r *http.Request) { a.libraryPage(w, r, tokenKind) }
func (a *App) UploadToken(w http.ResponseWriter, r *http.Request)     { a.uploadLibrary(w, r, tokenKind) }
func (a *App) ReplaceToken(w http.ResponseWriter, r *http.Request)    { a.replaceLibrary(w, r, tokenKind) }
func (a *App) RenameToken(w http.ResponseWriter, r *http.Request) {
	a.renameLibrary(w, r, tokenKind.assetKind)
}
func (a *App) DeleteToken(w http.ResponseWriter, r *http.Request) {
	a.deleteLibrary(w, r, tokenKind.assetKind)
}




func libraryCard(kind libraryKind, row queries.Asset) pages.LibraryAsset {
	card := pages.LibraryAsset{
		ID:       row.ID.String(),
		Name:     row.Name,
		FileName: row.FileName,
		Kind:     kind.Slug,
	}
	if row.Width.Valid {
		card.Width = int(row.Width.Int32)
	}
	if row.Height.Valid {
		card.Height = int(row.Height.Int32)
	}

	return card
}

func (a *App) libraryPage(w http.ResponseWriter, r *http.Request, kind libraryKind) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	cards, err := a.libraryList(ctx, sess.UserID, kind, "")
	if err != nil {
		slog.Error("Failed to load library assets", "error", err, "kind", kind.Slug)
		redirectToError(w, r)
		return
	}

	render(w, r, kind.Page(cards))
}










func (a *App) libraryList(ctx context.Context, ownerID ulid.ULID, kind libraryKind, term string) ([]pages.LibraryAsset, error) {
	var rows []queries.Asset
	var err error

	if term == "" {
		rows, err = a.Queries.GetLibraryAssets(ctx, queries.GetLibraryAssetsParams{
			OwnerID: ownerID,
			Type:    kind.Type,
		})
	} else {
		rows, err = a.Queries.SearchLibraryAssets(ctx, queries.SearchLibraryAssetsParams{
			OwnerID: ownerID,
			Type:    kind.Type,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}

	cards := make([]pages.LibraryAsset, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, libraryCard(kind, row))
	}

	return cards, nil
}












func (a *App) uploadLibrary(w http.ResponseWriter, r *http.Request, kind libraryKind) {
	card, ok := a.storeLibraryAsset(w, r, kind)
	if !ok {
		return
	}

	htmx.Toast(w, card.Name+" uploaded.")
	render(w, r, pages.LibraryAssetCard(card))
}





func (a *App) storeLibraryAsset(w http.ResponseWriter, r *http.Request, kind libraryKind) (pages.LibraryAsset, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	src, filename, ok := readImageUpload(w, r, "image", imageLimits)
	if !ok {
		return pages.LibraryAsset{}, false
	}

	encoded, width, height, err := kind.encode(src)
	if err != nil {
		slog.Error("Failed to encode library asset as webp", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return pages.LibraryAsset{}, false
	}

	assetID := ulid.Make()
	key := kind.Key(sess.UserID, assetID)
	name := assetName(filename)

	err = a.Queries.InsertLibraryAsset(ctx, queries.InsertLibraryAssetParams{
		ID:        assetID,
		OwnerID:   sess.UserID,
		FilePath:  key,
		Type:      kind.Type,
		FileName:  name,
		Name:      name,
		Width:     sql.NullInt32{Int32: int32(width), Valid: true},
		Height:    sql.NullInt32{Int32: int32(height), Valid: true},
		SizeBytes: int64(len(encoded)),
	})
	if err != nil {
		slog.Error("Failed to insert library asset", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return pages.LibraryAsset{}, false
	}

	if err := a.Storage.UploadImage(ctx, key, encoded); err != nil {
		slog.Error("Failed to upload library asset", "error", err, "kind", kind.Slug)
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.Delete(c, key)
		})
		htmx.ServerError(w)
		return pages.LibraryAsset{}, false
	}

	return pages.LibraryAsset{
		ID:       assetID.String(),
		Name:     name,
		FileName: name,
		Kind:     kind.Slug,
		Width:    width,
		Height:   height,
	}, true
}



















func (a *App) replaceLibrary(w http.ResponseWriter, r *http.Request, kind libraryKind) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, ok := a.libraryAsset(w, r, kind.assetKind)
	if !ok {
		return
	}

	src, filename, ok := readImageUpload(w, r, "image", imageLimits)
	if !ok {
		return
	}

	encoded, width, height, err := kind.encode(src)
	if err != nil {
		slog.Error("Failed to encode library asset as webp", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	if err := a.Storage.UploadImage(ctx, row.FilePath, encoded); err != nil {
		slog.Error("Failed to upload library asset", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	name := assetName(filename)
	err = a.Queries.ReplaceLibraryAsset(ctx, queries.ReplaceLibraryAssetParams{
		ID:        row.ID,
		OwnerID:   sess.UserID,
		Type:      kind.Type,
		FileName:  name,
		Width:     sql.NullInt32{Int32: int32(width), Valid: true},
		Height:    sql.NullInt32{Int32: int32(height), Valid: true},
		SizeBytes: int64(len(encoded)),
	})
	if err != nil {
		slog.Error("Failed to update library asset", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	
	
	
	htmx.Toast(w, name+" uploaded.")
	render(w, r, pages.LibraryAssetCard(pages.LibraryAsset{
		ID:       row.ID.String(),
		Name:     row.Name,
		FileName: name,
		Kind:     kind.Slug,
		Width:    width,
		Height:   height,
	}))
}








func (a *App) renameLibrary(w http.ResponseWriter, r *http.Request, kind assetKind) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, kind.One)
		return
	}

	name := assetName(strings.TrimSpace(r.FormValue("name")))
	if name == "" {
		name = "Untitled"
	}

	err = a.Queries.UpdateAssetName(ctx, queries.UpdateAssetNameParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Type:    kind.Type,
		Name:    name,
	})
	if err != nil {
		slog.Error("Failed to update library asset name", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" updated.")
}











func (a *App) deleteLibrary(w http.ResponseWriter, r *http.Request, kind assetKind) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, ok := a.libraryAsset(w, r, kind)
	if !ok {
		return
	}

	if err := a.Storage.Delete(ctx, row.FilePath); err != nil {
		slog.Error("Failed to delete library asset object", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	err := a.Queries.DeleteAsset(ctx, queries.DeleteAssetParams{
		ID:      row.ID,
		OwnerID: sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete library asset row", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, row.Name+" deleted.")
}








func (a *App) libraryAsset(w http.ResponseWriter, r *http.Request, kind assetKind) (queries.Asset, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, kind.One)
		return queries.Asset{}, false
	}

	row, err := a.Queries.GetLibraryAsset(ctx, queries.GetLibraryAssetParams{
		ID:      assetID,
		OwnerID: sess.UserID,
		Type:    kind.Type,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, kind.One)
		return queries.Asset{}, false
	}
	if err != nil {
		slog.Error("Failed to load library asset", "error", err, "kind", kind.Slug)
		htmx.ServerError(w)
		return queries.Asset{}, false
	}

	return row, true
}









func (k libraryKind) encode(src image.Image) ([]byte, int, int, error) {
	stored := k.Store(src)
	bounds := stored.Bounds()

	encoded, err := images.EncodeWebP(stored)
	if err != nil {
		return nil, 0, 0, err
	}

	return encoded, bounds.Dx(), bounds.Dy(), nil
}
