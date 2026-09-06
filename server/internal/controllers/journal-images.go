package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"

	"github.com/disintegration/imaging"
	"github.com/oklog/ulid/v2"
)

// Images inside a journal entry: two routes and the reconciliation that ties
// them to what the writer actually kept.
//
// AN IMAGE IS UPLOADED BEFORE IT IS REFERENCED. It is stored the moment it is
// pasted, dropped or picked, ahead of the save that will mention it, so adding
// a picture is two requests: this one takes the bytes and hands back a URL, and
// the entry's next debounced save carries that URL in the body. The row is born
// detached and that save is what attaches it, which is also what makes an
// upload the writer abandoned -- tab closed inside the debounce -- indis-
// tinguishable from an image they later removed. Both are swept.
//
// THE UPLOAD IS THE ONE ROUTE IN THE APP THAT DOES NOT ANSWER HTMX. Its caller
// is journal-editor.js inserting a node into the document rather than htmx
// swapping markup, so a success is a 201 with a Location header naming the
// image's URL, an empty body and no toast. The failures are still the ordinary
// htmx.Error helpers: they write the alert into an HX-Trigger header, and the
// editor's fetch reads that header itself and dispatches the event htmx would
// have dispatched for it -- so an upload that fails opens the same dialog every
// other failure in the app opens.
//
// NOTHING HERE DELETES AN IMAGE. Removing one from an entry detaches its row
// and internal/sweep takes it a day later; that is the whole removal path, and
// it is what keeps deleting an entry or a character one statement rather than
// one per picture. The single exception is an upload that failed after its row
// was written, which discardJournalImage rolls back below -- those bytes were
// never referenced by anything and have no undo to protect.
const (
	// journalImageEdge is the box every image is fitted inside, never
	// upscaled. It is generous for the column the entry renders in and mean
	// enough that a 12 megapixel phone photograph stops being one.
	journalImageEdge = 1600

	// journalImageLimit is images per entry, and it is a guard against a
	// runaway paste rather than an invariant -- see UploadJournalImage for
	// what it does and does not promise.
	journalImageLimit = 40

	// journalImageNameLimit is what assets.name holds. MySQL runs strict, so a
	// longer value is a driver error rather than a truncation, and it is
	// counted in runes because VARCHAR counts characters -- the same measure
	// the entry title uses.
	journalImageNameLimit = 255
)

// journalImagePath is the URL an entry's markdown carries for one of its
// images, and the string a save looks for to know the image is still in use.
//
// THE CHARACTER AND THE ENTRY ARE IN IT, and not because the serve route needs
// them to find the row -- the asset id alone would do that. They are there so
// the route can check every one of them against the row it found, which is what
// will let a share grant on one entry expose exactly that entry's images and
// nothing else the owner has. A URL of /assets/images/{id} could not be scoped
// to anything narrower than the whole account.
func journalImagePath(characterID, entryID, assetID ulid.ULID) string {
	return "/characters/" + characterID.String() + "/journal/" + entryID.String() + "/images/" + assetID.String()
}

// UploadJournalImage stores one image for one entry and answers with its URL.
//
// THE ORDER OF THE FIRST TWO STEPS IS THE POINT. The count is the ownership
// check as well as the cap, and it runs before the multipart body is touched,
// so a request naming somebody else's entry decodes nothing, allocates nothing
// and writes nothing. Doing it the other way round would mean a stranger could
// spend the server's memory on a 40 megapixel decode to be told 404.
func (a *App) UploadJournalImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	entryID, ok := journalEntryID(w, r)
	if !ok {
		return
	}

	// No matching journal is no row at all rather than a count of zero -- see
	// CountJournalImages for the GROUP BY that makes that true, and for why a
	// count of zero would otherwise be indistinguishable from a stranger's
	// entry.
	images, err := a.Queries.CountJournalImages(ctx, queries.CountJournalImagesParams{
		ID:          entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "journal entry")
		return
	}
	if err != nil {
		slog.Error("Failed to count journal images", "error", err)
		htmx.ServerError(w)
		return
	}
	// TWO UPLOADS RACING PAST THIS CAN LAND 41, and that is accepted. The cap
	// exists so a folder of photographs dropped on the editor at once is
	// refused rather than stored; nothing downstream depends on the number, so
	// a count read outside a transaction is the right amount of care for it.
	if images >= journalImageLimit {
		htmx.Error(w, "Too Many Images", "An entry can hold 40 images. Remove one to add another.", http.StatusUnprocessableEntity)
		return
	}

	src, filename, ok := readImageUpload(w, r, "image")
	if !ok {
		return
	}
	// Fit scales down to the box and hands back a copy unchanged when the
	// image already fits, so a screenshot narrower than the column is never
	// resampled and nothing is ever enlarged.
	encoded, err := encodeWebP(imaging.Fit(src, journalImageEdge, journalImageEdge, imaging.Lanczos))
	if err != nil {
		slog.Error("Failed to encode journal image as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	assetID := ulid.Make()
	name := journalImageName(filename)
	// NOTE: the row is the ledger for what lives in R2, so it is written first
	// and rolled back if the upload never lands
	err = a.Queries.InsertJournalImage(ctx, queries.InsertJournalImageParams{
		ID:        assetID,
		OwnerID:   sess.UserID,
		JournalID: &entryID,
		FilePath:  storage.JournalImageKey(sess.UserID, assetID),
		FileName:  name,
		Name:      name,
	})
	if err != nil {
		slog.Error("Failed to insert journal image", "error", err)
		htmx.ServerError(w)
		return
	}

	if err := a.Storage.UploadJournalImage(ctx, sess.UserID, assetID, encoded); err != nil {
		slog.Error("Failed to upload journal image", "error", err)
		a.discardJournalImage(ctx, sess.UserID, assetID)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Location", journalImagePath(characterID, entryID, assetID))
	w.WriteHeader(http.StatusCreated)
}

// GetJournalImage is the read, and it is the only one. GetJournalImage in
// journals.sql carries the asset, the entry, the character and the owner, so
// every id in the URL is checked against the row rather than trusted; the
// deliberately unscoped GetImage behind /assets/images/{id} does not serve this
// type and must never be taught to.
//
// IT IS CACHED FOREVER, unlike serveImage. A journal image is never replaced at
// its URL -- there is no replace route, and the sweeper deletes rather than
// overwrites -- so the bytes behind an id cannot change and the browser has
// nothing to revalidate. private, because which reader is asking is what
// decides whether there is an answer at all.
//
// RequireSessionOr404 rather than RequireSession, like the other image routes:
// a redirect to the sign-in page renders as a broken image.
func (a *App) GetJournalImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	// All three parsed before anything is queried, and a failure is
	// http.NotFound rather than htmx.NotFound: the caller is an <img>, which
	// has nothing to show an alert in and no swap to make.
	characterID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	entryID, err := ulid.Parse(r.PathValue("entryId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	assetID, err := ulid.Parse(r.PathValue("assetId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	key, err := a.Queries.GetJournalImage(ctx, queries.GetJournalImageParams{
		AssetID:     assetID,
		EntryID:     entryID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load journal image row", "error", err, "assetID", assetID.String())
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	a.streamImage(w, r, key, `"`+assetID.String()+`"`)
}

// reconcileJournalImages brings the entry's image rows into line with the body
// that was just saved: an image whose URL is in the body is attached, one whose
// URL is not is detached, and only the rows that have to flip are written. In
// the steady state -- a writer typing prose around pictures that are already
// there -- it is one indexed read of a few dozen rows and no writes at all.
//
// THE REFERENCE TEST IS A SUBSTRING SEARCH for the URL, not a markdown parse,
// and it is exact in the direction that matters: an image cannot render without
// its URL appearing in the body, so nothing in use is ever detached. In the
// other direction it over-counts -- a URL inside a code fence keeps its image
// alive -- and that costs one object for as long as the fence stays, which is
// not worth a markdown parser in the save path to avoid.
//
// BEST EFFORT, AND AFTER THE WRITE. The save has already happened and has
// already been answered as far as the writer is concerned, so a failure here is
// logged and nothing else: the next debounce reconciles again, and the sweeper
// only ever acts on a row that has been detached for a day.
func (a *App) reconcileJournalImages(ctx context.Context, characterID, entryID, ownerID ulid.ULID, body string) {
	states, err := a.Queries.ListJournalImageStates(ctx, queries.ListJournalImageStatesParams{
		JournalID: &entryID,
		OwnerID:   ownerID,
	})
	if err != nil {
		slog.Error("Failed to list journal images", "error", err, "entryID", entryID.String())
		return
	}

	attach, detach := journalImageFlips(states, func(assetID ulid.ULID) bool {
		return strings.Contains(body, journalImagePath(characterID, entryID, assetID))
	})

	for _, assetID := range attach {
		err := a.Queries.AttachJournalImage(ctx, queries.AttachJournalImageParams{
			ID:      assetID,
			OwnerID: ownerID,
		})
		if err != nil {
			slog.Error("Failed to attach journal image", "error", err, "assetID", assetID.String())
		}
	}
	for _, assetID := range detach {
		err := a.Queries.DetachJournalImage(ctx, queries.DetachJournalImageParams{
			ID:      assetID,
			OwnerID: ownerID,
		})
		if err != nil {
			slog.Error("Failed to detach journal image", "error", err, "assetID", assetID.String())
		}
	}
}

// journalImageFlips reports which of an entry's images have to change state for
// the body: attach is the detached images the body references, detach the
// attached ones it does not. An image already in the state the body implies is
// in neither list.
//
// It is separate from the I/O above so the decision can be tested without a
// database, which is the half worth testing -- the other half is two loops
// running one statement each.
func journalImageFlips(states []queries.ListJournalImageStatesRow, referenced func(ulid.ULID) bool) (attach, detach []ulid.ULID) {
	for _, state := range states {
		// detached_at is the state: a valid one means the row is detached and
		// the sweeper is counting down on it.
		switch inBody, attached := referenced(state.ID), !state.DetachedAt.Valid; {
		case inBody && !attached:
			attach = append(attach, state.ID)
		case !inBody && attached:
			detach = append(detach, state.ID)
		}
	}

	return attach, detach
}

// journalImageName cuts the uploaded filename to what the column holds. Nothing
// renders it: a pasted clipboard image arrives as image.png or with no name at
// all, and the value is there so a row in the bucket's ledger can be recognised
// by a person reading the table.
func journalImageName(filename string) string {
	runes := []rune(filename)
	if len(runes) > journalImageNameLimit {
		return string(runes[:journalImageNameLimit])
	}

	return filename
}

// discardJournalImage rolls back an upload that failed after its row was
// written, on the same terms as discardAvatar: the row is only dropped once R2
// confirms the object is gone, so a cleanup failure leaves it behind as the
// record that the object may still exist.
//
// This is the one place outside internal/sweep that deletes a journal image,
// and the exception is narrow: the bytes it removes were never referenced by an
// entry, so there is no undo to keep them for.
func (a *App) discardJournalImage(ctx context.Context, userID ulid.ULID, assetID ulid.ULID) {
	cleanupCtx, cancel := storage.CleanupContext(ctx)
	defer cancel()

	if err := a.Storage.Delete(cleanupCtx, storage.JournalImageKey(userID, assetID)); err != nil {
		slog.Error("Failed to clean up journal image object; leaving the asset row behind", "error", err, "assetID", assetID.String())
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
