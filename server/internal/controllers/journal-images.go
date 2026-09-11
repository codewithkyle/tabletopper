package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"

	"github.com/disintegration/imaging"
	"github.com/oklog/ulid/v2"
)



























const (
	
	
	
	journalImageEdge = 1600

	
	
	
	journalImageLimit = 40
)










func journalImagePath(characterID, entryID, assetID ulid.ULID) string {
	return journalImagePrefix(characterID, entryID) + assetID.String()
}








func journalImagePrefix(characterID, entryID ulid.ULID) string {
	return "/characters/" + characterID.String() + "/journal/" + entryID.String() + "/images/"
}








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

	
	
	
	
	held, err := a.Queries.CountJournalImages(ctx, queries.CountJournalImagesParams{
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
	
	
	
	
	if held >= journalImageLimit {
		htmx.Error(w, "Too Many Images", "An entry can hold 40 images. Remove one to add another.", http.StatusUnprocessableEntity)
		return
	}

	src, filename, ok := readImageUpload(w, r, "image", imageLimits)
	if !ok {
		return
	}
	
	
	
	encoded, err := images.EncodeWebP(imaging.Fit(src, journalImageEdge, journalImageEdge, imaging.Lanczos))
	if err != nil {
		slog.Error("Failed to encode journal image as webp", "error", err)
		htmx.ServerError(w)
		return
	}

	assetID := ulid.Make()
	name := assetName(filename)
	
	
	err = a.Queries.InsertJournalImage(ctx, queries.InsertJournalImageParams{
		ID:        assetID,
		OwnerID:   sess.UserID,
		JournalID: &entryID,
		FilePath:  storage.JournalImageKey(sess.UserID, assetID),
		FileName:  name,
		Name:      name,
		SizeBytes: int64(len(encoded)),
	})
	if err != nil {
		slog.Error("Failed to insert journal image", "error", err)
		htmx.ServerError(w)
		return
	}

	if err := a.Storage.UploadJournalImage(ctx, sess.UserID, assetID, encoded); err != nil {
		slog.Error("Failed to upload journal image", "error", err)
		a.discardAsset(ctx, sess.UserID, assetID, func(c context.Context) error {
			return a.Storage.Delete(c, storage.JournalImageKey(sess.UserID, assetID))
		})
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Location", journalImagePath(characterID, entryID, assetID))
	w.WriteHeader(http.StatusCreated)
}















func (a *App) GetJournalImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	
	
	
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









func journalImageFlips(states []queries.ListJournalImageStatesRow, referenced func(ulid.ULID) bool) (attach, detach []ulid.ULID) {
	for _, state := range states {
		
		
		switch inBody, attached := referenced(state.ID), !state.DetachedAt.Valid; {
		case inBody && !attached:
			attach = append(attach, state.ID)
		case !inBody && attached:
			detach = append(detach, state.ID)
		}
	}

	return attach, detach
}
