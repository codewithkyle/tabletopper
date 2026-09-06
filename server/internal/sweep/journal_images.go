// Package sweep is the background half of removal: it deletes what a request
// deliberately did not.
//
// Taking a picture out of a journal entry marks its asset row detached and
// stops there. Deleting the entry, or the whole character, does the same. This
// package is what then deletes the object and the row, a day later, and it is
// the only code in the app that deletes a journal image.
//
// THE DELAY IS THE FEATURE. It is the window an undo has: a writer who cut a
// picture in the evening and wants it back in the morning finds it still
// serving, because nothing in a request ever removed it. It is also what keeps
// every delete one statement -- an entry holding forty images goes in the same
// time as an empty one, and no request is ever held open on R2 while a bucket
// is tidied.
//
// AN ACCEPTED RACE LIVES HERE, and it is the reason the objects go before the
// rows. A save that re-attaches an image between the batch being read and its
// row being deleted keeps the row, but the object is already gone and that
// image is broken. The window is the milliseconds of one batch, and landing in
// it takes an undo of an image detached a day earlier arriving exactly then.
// The other order trades that for a bucket quietly filling with objects no row
// remembers, which nothing would ever find again -- and the row being the
// record that an object may exist is the rule everywhere else in this app.
package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
)

const (
	// journalImageInterval is how often detached journal images are swept.
	journalImageInterval = time.Hour

	// journalImageGrace is how long a detached image stays before it is
	// deleted. It is the window an undo has, and it has to cover a writer who
	// removed a picture in the evening and wanted it back the next morning.
	journalImageGrace = 24 * time.Hour

	// journalImageBatch is the LIMIT inside ListSweepableJournalImages, which
	// sqlc gives no parameter for. It is repeated here because a batch shorter
	// than this is how a pass knows the backlog has drained, so the two have to
	// be kept in step.
	journalImageBatch = 100
)

// JournalImages deletes detached journal images in the background until ctx is
// cancelled, once at start and then hourly -- the shape session.StartCleanup
// uses, for the same reason: a process that has just come up should not wait an
// hour to do the work it was down for.
func JournalImages(ctx context.Context, q *queries.Queries, store *storage.Client) {
	go func() {
		ticker := time.NewTicker(journalImageInterval)
		defer ticker.Stop()

		sweepJournalImages(ctx, q, store)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Journal image sweep stopped")
				return
			case <-ticker.C:
				sweepJournalImages(ctx, q, store)
			}
		}
	}()
}

// sweepJournalImages is one pass: batches of the oldest detached images until
// one comes back short.
//
// THE CUTOFF IS TAKEN ONCE and carried through the whole pass, the read and
// every delete. The delete repeats the condition rather than trusting the read,
// so an image a save re-attached in between keeps its row; a fresh cutoff there
// would be a different question asked of a different instant.
func sweepJournalImages(ctx context.Context, q *queries.Queries, store *storage.Client) {
	cutoff := sql.NullTime{Time: time.Now().Add(-journalImageGrace), Valid: true}

	swept := int64(0)
	for {
		batch, err := q.ListSweepableJournalImages(ctx, cutoff)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to list sweepable journal images", "error", err)
			return
		}
		if len(batch) == 0 {
			break
		}

		keys := make([]string, 0, len(batch))
		for _, image := range batch {
			keys = append(keys, image.FilePath)
		}
		// One call for the batch, and a failure abandons the pass without
		// touching a row. The rows are the ledger: leaving all of them means
		// the keys that did go are simply deleted again next hour, which
		// succeeds, because deleting a key that is not there succeeds.
		if err := store.DeleteMany(ctx, keys); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to delete journal image objects", "error", err, "count", len(keys))
			return
		}

		deleted := int64(0)
		for _, image := range batch {
			result, err := q.DeleteSweptJournalImage(ctx, queries.DeleteSweptJournalImageParams{
				ID:     image.ID,
				Cutoff: cutoff,
			})
			if err != nil {
				slog.Error("Failed to delete swept journal image row", "error", err, "assetID", image.ID.String())
				continue
			}
			if rows, err := result.RowsAffected(); err == nil {
				deleted += rows
			}
		}
		swept += deleted

		// A FULL BATCH THAT REMOVED NOTHING IS NOT A BACKLOG, it is the deletes
		// failing, and the next read would hand back the same hundred rows for
		// the same treatment -- a loop that would spin on R2 and the database
		// until the process stopped. Ending the pass costs an hour and the next
		// one starts from the same place.
		if deleted == 0 || len(batch) < journalImageBatch {
			break
		}
	}

	if swept > 0 {
		slog.Info("Swept journal images", "count", swept)
	}
}
