package sweep

import (
	"context"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
)

const (
	// musicUploadInterval is how often abandoned music uploads are collected.
	musicUploadInterval = time.Hour

	// musicUploadGrace is how long a row that has not been confirmed is left
	// alone. IT IS NOT A GRACE PERIOD FOR AN UNDO, unlike the journal's next
	// door -- there is nothing here anybody would want back. It is a margin for
	// an upload that is merely SLOW.
	//
	// A track may be 175 MB and the signed URL is good for an hour, so an
	// upload that is still running an hour after its row was written is
	// possible; one still running six hours later is a browser that was closed
	// with the tab open, or a PUT that R2 refused. Six hours is well clear of
	// the first and short enough that a bucket does not quietly fill with the
	// second.
	musicUploadGrace = 6 * time.Hour

	// musicUploadBatch is the LIMIT inside ListAbandonedMusicUploads, which
	// sqlc gives no parameter for. It is repeated here because a batch shorter
	// than this is how a pass knows the backlog has drained, so the two have to
	// be kept in step.
	musicUploadBatch = 100
)

// MusicUploads deletes music rows whose upload never finished, in the background
// until ctx is cancelled, once at start and then hourly.
//
// IT EXISTS BECAUSE A MUSIC UPLOAD IS TWO REQUESTS AND THE SECOND ONE IS
// OPTIONAL. Every other upload in this app finishes inside the request that
// began it, so a failure has a handler standing there to roll it back. A track
// goes browser-to-bucket: the row is written, a signed URL is handed out, and
// then this server hears nothing until a confirm arrives -- which it never does
// if the tab was closed, the laptop shut, or the PUT refused. Nothing in a
// request could ever notice, because the request that would have noticed is the
// one that did not happen.
//
// SO WHAT IS BEING COLLECTED IS A ROW THAT MAY OR MAY NOT OWN AN OBJECT. The
// object goes first and the row only once R2 has confirmed it is gone, which is
// the order every delete in this app uses; deleting a key that was never written
// succeeds, so the two cases need no telling apart.
func MusicUploads(ctx context.Context, q *queries.Queries, store *storage.Client) {
	go func() {
		ticker := time.NewTicker(musicUploadInterval)
		defer ticker.Stop()

		sweepMusicUploads(ctx, q, store)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Abandoned music upload sweep stopped")
				return
			case <-ticker.C:
				sweepMusicUploads(ctx, q, store)
			}
		}
	}()
}

func sweepMusicUploads(ctx context.Context, q *queries.Queries, store *storage.Client) {
	for {
		cutoff := time.Now().Add(-musicUploadGrace)

		rows, err := q.ListAbandonedMusicUploads(ctx, cutoff)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to list abandoned music uploads", "error", err)
			return
		}
		if len(rows) == 0 {
			return
		}

		for _, row := range rows {
			if err := store.Delete(ctx, row.FilePath); err != nil {
				if ctx.Err() != nil {
					return
				}
				// The row stays, so the next pass tries again. It is the record
				// that an object may exist, and dropping it here would strand
				// whatever is in the bucket with nothing left to name it.
				slog.Error("Failed to delete an abandoned track; leaving its row", "error", err, "assetID", row.ID.String())
				continue
			}

			err := q.DeleteAsset(ctx, queries.DeleteAssetParams{ID: row.ID, OwnerID: row.OwnerID})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Failed to delete an abandoned track's row", "error", err, "assetID", row.ID.String())
			}
		}

		slog.Info("Swept abandoned music uploads", "count", len(rows))

		// A short batch is the whole backlog, so there is nothing left to do
		// until the next tick.
		if len(rows) < musicUploadBatch {
			return
		}
	}
}
