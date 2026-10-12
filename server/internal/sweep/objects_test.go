package sweep

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

type fakePurges struct {
	queue    []queries.ListObjectPurgesRow
	lists    int
	deleted  []ulid.ULID
	attempts []ulid.ULID
	order    []string
}

func (f *fakePurges) ListObjectPurges(context.Context) ([]queries.ListObjectPurgesRow, error) {
	f.lists++
	batch := f.queue[:min(len(f.queue), objectPurgeBatch)]
	return append([]queries.ListObjectPurgesRow(nil), batch...), nil
}
func (f *fakePurges) DeleteObjectPurge(_ context.Context, id ulid.ULID) error {
	f.deleted = append(f.deleted, id)
	f.order = append(f.order, "row "+id.String())
	for i, row := range f.queue {
		if row.ID == id {
			f.queue = append(f.queue[:i], f.queue[i+1:]...)
			break
		}
	}
	return nil
}
func (f *fakePurges) RecordObjectPurgeAttempt(_ context.Context, id ulid.ULID) error {
	f.attempts = append(f.attempts, id)
	return nil
}

type fakeBucket struct {
	purges   *fakePurges
	keys     []string
	prefixes []string
	failOn   string
}

func (b *fakeBucket) Delete(_ context.Context, key string) error {
	if b.failOn != "" && strings.Contains(key, b.failOn) {
		return errors.New("the bucket refused " + key)
	}
	b.keys = append(b.keys, key)
	b.purges.order = append(b.purges.order, "object "+key)
	return nil
}
func (b *fakeBucket) DeletePrefix(_ context.Context, prefix string) error {
	if b.failOn != "" && strings.Contains(prefix, b.failOn) {
		return errors.New("the bucket refused " + prefix)
	}
	b.prefixes = append(b.prefixes, prefix)
	b.purges.order = append(b.purges.order, "object "+prefix)
	return nil
}
func queued(keys ...string) *fakePurges {
	f := &fakePurges{}
	for _, key := range keys {
		f.queue = append(f.queue, queries.ListObjectPurgesRow{
			ID:        ulid.Make(),
			ObjectKey: key,
			IsPrefix:  strings.HasSuffix(key, "/"),
		})
	}
	return f
}
func TestAPurgeDeletesTheObjectBeforeItsRow(t *testing.T) {
	purges := queued("users/u/tokens/a")
	id := purges.queue[0].ID
	bucket := &fakeBucket{purges: purges}
	purgeObjects(context.Background(), purges, bucket)
	want := []string{"object users/u/tokens/a", "row " + id.String()}
	if strings.Join(purges.order, ",") != strings.Join(want, ",") {
		t.Errorf("the purge ran %v, want %v", purges.order, want)
	}
	if len(purges.queue) != 0 {
		t.Errorf("%d rows are still queued", len(purges.queue))
	}
}
func TestAPrefixRowDeletesThePrefix(t *testing.T) {
	purges := queued("users/u/maps/m/", "users/u/tokens/a")
	bucket := &fakeBucket{purges: purges}
	purgeObjects(context.Background(), purges, bucket)
	if len(bucket.prefixes) != 1 || bucket.prefixes[0] != "users/u/maps/m/" {
		t.Errorf("prefixes deleted = %v, want the map's", bucket.prefixes)
	}
	if len(bucket.keys) != 1 || bucket.keys[0] != "users/u/tokens/a" {
		t.Errorf("keys deleted = %v, want the token's", bucket.keys)
	}
}
func TestAnObjectThatWillNotDeleteKeepsItsRowAndCountsTheAttempt(t *testing.T) {
	purges := queued("users/u/tokens/a", "users/u/tokens/stuck")
	stuck := purges.queue[1].ID
	bucket := &fakeBucket{purges: purges, failOn: "stuck"}
	purgeObjects(context.Background(), purges, bucket)
	if len(purges.queue) != 1 || purges.queue[0].ID != stuck {
		t.Fatalf("queued after the pass = %v, want only the stuck row", purges.queue)
	}
	if len(purges.attempts) != 1 || purges.attempts[0] != stuck {
		t.Errorf("attempts recorded for %v, want the stuck row once", purges.attempts)
	}
	if purges.lists != 1 {
		t.Errorf("listed %d times; a pass with a failure must not go round again", purges.lists)
	}
}
func TestAFullBatchIsFollowedByTheNext(t *testing.T) {
	keys := make([]string, objectPurgeBatch+5)
	for i := range keys {
		keys[i] = "users/u/journals/" + ulid.Make().String()
	}
	purges := queued(keys...)
	bucket := &fakeBucket{purges: purges}
	purgeObjects(context.Background(), purges, bucket)
	if len(purges.queue) != 0 {
		t.Errorf("%d rows were left for the next pass", len(purges.queue))
	}
	if len(bucket.keys) != len(keys) {
		t.Errorf("deleted %d objects, want %d", len(bucket.keys), len(keys))
	}
}
func TestPurgeRefusesWhatItCouldNeverDelete(t *testing.T) {
	for name, object := range map[string]Object{
		"an empty key":               Key(""),
		"a prefix without its slash": Prefix("users/u/maps/m"),
		"an empty prefix":            Prefix(""),
	} {
		if err := Purge(context.Background(), nil, object); err == nil {
			t.Errorf("%s was queued", name)
		}
	}
}
