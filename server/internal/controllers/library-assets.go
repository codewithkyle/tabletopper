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

// THE LIBRARY IS THE KINDS THE ACCOUNT GATHERS, and every one of them is the
// same five handlers with a different word in them: list, upload, replace,
// rename, delete.
//
// SO THERE IS ONE SET OF HANDLERS AND A DESCRIPTOR PER KIND. The alternative
// was ten near-identical methods, which is ten places to fix the day the
// rollback order changes -- and that order is the property this file exists to
// protect. Every one of these writes the row before the object and rolls the
// row back if the object never lands, because the row is the ledger for what
// lives in R2; an object with no row is a file nothing remembers, and a row
// with no object is a broken picture that renders forever.
//
// MAPS ARE NOT ONE OF THESE and are not going to be. A map is a tile pyramid
// and a background job as well as a row: its upload queues work, its replace
// leaves the old pyramid serving, and its delete removes a prefix rather than a
// key. Folding it in would mean four branches on kind inside every handler
// here, which is the copy this consolidation was avoiding, written vertically.
//
// MUSIC WILL NOT BE ONE EITHER, for a plainer reason: it is not an image, so
// nothing from readImageUpload down applies to it, and its bytes never pass
// through this process at all.

const (
	// avatarLibrarySize is what a library avatar is stored at, and it is the
	// monster picture's 256 rather than the character portrait's 96.
	//
	// THE TWO ARE SEPARATE CONSTANTS BECAUSE THE PICTURES HAVE SEPARATE JOBS. A
	// portrait is a thumbnail: it appears on a roster card and in a sheet's bar
	// and is never drawn larger, so 96 is the size it is used at. A library
	// avatar is the face a GM spawns an NPC with, so it ends up on a map at
	// whatever zoom the table is at -- which is the argument monsterImageSize
	// makes, and it lands on the same number.
	//
	// 96 WAS ALSO VISIBLY SOFT ON THE MANAGER PAGE. A card cannot be narrower
	// than its own controls, so the densest the Avatars wall goes is about 11
	// rems, and a 96-pixel image in a 144-pixel box is upscaled before a HiDPI
	// screen has doubled it.
	//
	// Raising it does not re-encode what is already in the bucket: a row keeps
	// the object it was written with until it is replaced.
	avatarLibrarySize = 256

	// tokenSize is the box a token is fitted into, and it is large because a
	// token is not a thumbnail: it is drawn on a map at whatever zoom the game
	// master is working at, like a monster's picture, and unlike that one it
	// may be long rather than square. 512 is the longest edge, so a wagon three
	// times as wide as it is tall is stored 512 by 171.
	tokenSize = 512
)

// assetKind is the least a handler can know about a kind and still act on one
// of its rows: which member to scope by, which segment it is routed under, and
// what to call it in a message.
//
// IT IS SPLIT OUT FROM libraryKind BECAUSE MUSIC USES HALF OF THESE HANDLERS.
// Renaming and deleting are the same work whatever the asset is -- a column and
// an object at file_path -- and music is not an image, so it has none of the
// resizing, no shared page and no upload in common with the two below. Giving
// it a libraryKind with nil in the image-shaped fields would have put a landmine
// in every one of them.
type assetKind struct {
	// Type is the assets.type member. It is passed to every statement, which
	// is what stops a token's id reaching an avatar's route and being renamed,
	// replaced or deleted through it.
	Type queries.AssetsType

	// Slug is the path segment its routes sit under, and the id of the grid its
	// cards are swapped into. The card builds its URLs from it.
	Slug string

	// One is the word for a single one of these, used by htmx.NotFound to say
	// "That token no longer exists" rather than naming a row.
	One string
}

// libraryKind is one kind of PICTURE the account gathers: an assetKind, plus
// where its objects live, how it is resized on the way in, and the page that
// lists it.
type libraryKind struct {
	assetKind

	// Key builds the object's key in the bucket.
	Key func(userID ulid.ULID, assetID ulid.ULID) string

	// Store is the resize, and it is THE ONE THING THE TWO KINDS ACTUALLY
	// DISAGREE ABOUT. An avatar is cropped square, because a face is in the
	// middle of the picture and a portrait is a thumbnail. A token is fitted
	// with its aspect kept, because a longboat cropped square is a square of
	// hull -- the shape is what is being stored.
	Store func(src image.Image) image.Image

	// Page renders the whole manager page for this kind, and Cards renders just
	// the grid inside it -- the same section, which is what the search fragment
	// swaps. They are two fields rather than one because the page is what a
	// browser navigates to and the grid is what htmx replaces; a fragment that
	// answered with the page would swap a whole document into a section.
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

	// Music, which reaches only the two handlers below that do not care what an
	// asset is made of. Everything else it needs is in music-assets.go.
	musicKind = assetKind{Type: queries.AssetsTypeMusic, Slug: "music", One: "track"}
)

// The ten routes, which are the five below with a kind bound to each. They are
// methods rather than closures built in routes.go so that the pattern and the
// handler still read as a pair there.
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

// libraryCard is the assets row as its card reads it. Six values out of
// twenty-two columns; every pyramid and job column is NULL for these kinds and
// none of them is asked about.
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

// libraryList is one kind's shelf in either of its two states -- everything the
// owner has, or what matched a search -- so the page and the search fragment
// build the same cards from the same function, and a card that appeared in only
// one of them cannot exist.
//
// THE TERM IS ESCAPED BY journalSearchPattern RATHER THAN BY A SECOND COPY OF
// IT. What LIKE reads as a pattern is a fact about MySQL and not about journals:
// an unescaped `%` matches the whole shelf here exactly as it matches the whole
// journal there.
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

// uploadLibrary takes one picture into the library.
//
// THE ROW IS WRITTEN BEFORE THE OBJECT AND ROLLED BACK IF THE OBJECT NEVER
// LANDS. The row is the ledger for what lives in R2 and it names the key the
// object will go to, so it has to exist first: an object written under a key no
// row remembers is a file nothing can render, nothing will delete, and no
// sweeper will find, because every sweep works from these rows.
//
// It answers with the card it just made, which is the mutation case the
// fragment rules name -- a POST replying with the thing it created. The
// alternative is a POST that returns nothing followed by a GET to fetch it.
func (a *App) uploadLibrary(w http.ResponseWriter, r *http.Request, kind libraryKind) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

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
		return
	}

	if err := a.Storage.UploadImage(ctx, key, encoded); err != nil {
		slog.Error("Failed to upload library asset", "error", err, "kind", kind.Slug)
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.Delete(c, key)
		})
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" uploaded.")
	render(w, r, pages.LibraryAssetCard(pages.LibraryAsset{
		ID:       assetID.String(),
		Name:     name,
		FileName: name,
		Kind:     kind.Slug,
		Width:    width,
		Height:   height,
	}))
}

// replaceLibrary swaps the picture behind an existing row.
//
// THE OBJECT IS OVERWRITTEN AT THE KEY THE ROW ALREADY NAMES, so nothing is
// orphaned, nothing needs sweeping, and the card's <img> src does not change --
// which is why serveImage sends no-cache rather than no-store, and why
// ReplaceLibraryAsset moves updated_at by hand. The ETag is built from that
// column, and a replacement that did not move it would go on serving the old
// picture out of the browser's cache at the same URL.
//
// THE KEY COMES OFF THE ROW AND IS NOT REBUILT. For a token or an avatar
// uploaded since the library existed these are the same string, but reading it
// is the rule the whole app follows -- file_path is where an object's location
// is written down, and rebuilding it is how a key gets out of step with the
// bucket.
//
// Ownership is checked before anything is written, because the key is built
// from the session's user id: writing first would land a stranger's asset id in
// this user's namespace with no row behind it.
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

	// The asset keeps its own name. Replacing the picture behind a token called
	// "Rowboat" with rowboat-v2.png does not rename it to rowboat-v2.png; only
	// the filename chip changes, which is what the chip is for.
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

// renameLibrary writes the label the owner typed over the card's heading. It
// touches nothing in the bucket: a name is a column, and the object is found by
// file_path, which this never changes.
//
// The reply is a toast and nothing else -- hx-swap="none" on the box -- because
// the box already shows what was typed and swapping over it would move the
// caret while somebody was still in it.
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

// deleteLibrary removes one asset, OBJECT FIRST AND ROW LAST.
//
// That order is the one every delete in this app uses and it is the opposite of
// the upload's. The row is the record that an object may exist, so it may only
// go once R2 has confirmed that the object does not: a row deleted first would
// leave a file in the bucket that nothing remembers and nothing will ever find.
//
// The reply is 200 with an empty body and not 204 -- noSwap lists 204, and a
// status in that list overrides the hx-swap="delete" on the button, which would
// leave the card on screen after the asset was gone.
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

// libraryAsset parses the id and loads the row, scoped to the owner AND to the
// kind the route was for. It writes the response itself when there is nothing
// to find, so a caller only has to stop.
//
// THE KIND IN THE WHERE IS WHAT MAKES THE KIND IN THE PATH MEAN SOMETHING.
// Without it, a token's id sent to DELETE /assets/avatars/{id} would delete the
// token -- the row is the owner's either way, so nothing else would refuse it.
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

// encode resizes a decoded upload the way this kind stores it and hands back
// the bytes and the pixels they came out at.
//
// THE DIMENSIONS ARE MEASURED AFTER THE RESIZE, not before, which is the whole
// reason this is one function. What a renderer needs is the shape of the image
// it is about to draw, and for a token that is the fitted box rather than the
// upload -- a 3000 by 1000 wagon is stored 512 by 171, and 3000 by 1000 would
// be a true fact about a file nobody will ever fetch.
func (k libraryKind) encode(src image.Image) ([]byte, int, int, error) {
	stored := k.Store(src)
	bounds := stored.Bounds()

	encoded, err := images.EncodeWebP(stored)
	if err != nil {
		return nil, 0, 0, err
	}

	return encoded, bounds.Dx(), bounds.Dy(), nil
}
