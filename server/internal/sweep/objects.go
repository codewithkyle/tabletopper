package sweep

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

const (
	objectPurgeInterval = time.Minute
	objectPurgeBatch    = 100
)

type Object struct {
	Key    string
	Prefix bool
}

func Key(key string) Object       { return Object{Key: key} }
func Prefix(prefix string) Object { return Object{Key: prefix, Prefix: true} }

func Purge(ctx context.Context, q *queries.Queries, objects ...Object) error {
	for _, object := range objects {
		if object.Key == "" {
			return errors.New("purge: an empty key")
		}
		if object.Prefix && !strings.HasSuffix(object.Key, "/") {
			return fmt.Errorf("purge: %q is not a prefix", object.Key)
		}
		err := q.EnqueueObjectPurge(ctx, queries.EnqueueObjectPurgeParams{
			ID:        ulid.Make(),
			ObjectKey: object.Key,
			IsPrefix:  object.Prefix,
		})
		if err != nil {
			return fmt.Errorf("purge %s: %w", object.Key, err)
		}
	}
	return nil
}

type Bucket interface {
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
}
type purgeRows interface {
	ListObjectPurges(ctx context.Context) ([]queries.ListObjectPurgesRow, error)
	DeleteObjectPurge(ctx context.Context, id ulid.ULID) error
	RecordObjectPurgeAttempt(ctx context.Context, id ulid.ULID) error
}

func Objects(ctx context.Context, q *queries.Queries, store Bucket) {
	Every(ctx, "Object purge", objectPurgeInterval, func(ctx context.Context) {
		purgeObjects(ctx, q, store)
	})
}
func purgeObjects(ctx context.Context, rows purgeRows, store Bucket) {
	purged := 0
	defer func() {
		if purged > 0 {
			slog.Info("Purged objects", "count", purged)
		}
	}()
	for {
		batch, err := rows.ListObjectPurges(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("Failed to list objects to purge", "error", err)
			}
			return
		}
		failed := 0
		var cause error
		for _, row := range batch {
			if err := remove(ctx, store, row); err != nil {
				if ctx.Err() != nil {
					return
				}
				failed++
				cause = err
				if err := rows.RecordObjectPurgeAttempt(ctx, row.ID); err != nil && ctx.Err() == nil {
					slog.Error("Failed to record an object purge attempt", "error", err, "key", row.ObjectKey)
				}
				continue
			}
			if err := rows.DeleteObjectPurge(ctx, row.ID); err != nil {
				if ctx.Err() != nil {
					return
				}
				failed++
				cause = err
				continue
			}
			purged++
		}
		if failed > 0 {
			slog.Error("Failed to purge some objects; they will be tried again", "count", failed, "error", cause)
			return
		}
		if len(batch) < objectPurgeBatch {
			return
		}
	}
}
func remove(ctx context.Context, store Bucket, row queries.ListObjectPurgesRow) error {
	if row.IsPrefix {
		return store.DeletePrefix(ctx, row.ObjectKey)
	}
	return store.Delete(ctx, row.ObjectKey)
}
func inTx(ctx context.Context, db *sql.DB, fn func(q *queries.Queries) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(queries.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
