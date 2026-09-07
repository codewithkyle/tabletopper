package session

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"tabletopper/internal/queries"
)

// countingDB is a queries.DBTX that records how many statements were run
// through it. Only ExecContext is reachable from Refresh; the other three are
// here because DBTX declares them, and a call to one of them from this path
// would be a change worth failing on.
type countingDB struct {
	t     *testing.T
	execs int
}

func (d *countingDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	d.execs++

	return execResult{}, nil
}

func (d *countingDB) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	d.t.Fatal("Refresh prepared a statement")

	return nil, nil
}

func (d *countingDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	d.t.Fatal("Refresh ran a query")

	return nil, nil
}

func (d *countingDB) QueryRowContext(context.Context, string, ...any) *sql.Row {
	d.t.Fatal("Refresh ran a query")

	return nil
}

// execResult is an UPDATE that matched the one row it was aimed at.
type execResult struct{}

func (execResult) LastInsertId() (int64, error) { return 0, nil }
func (execResult) RowsAffected() (int64, error) { return 1, nil }

func refreshOnce(t *testing.T, refreshedAt time.Time) (*countingDB, *httptest.ResponseRecorder) {
	t.Helper()

	db := &countingDB{t: t}
	store := NewStore(queries.New(db), false)
	rec := httptest.NewRecorder()

	s := UserSession{
		Hash:        []byte("hash"),
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		RefreshedAt: refreshedAt,
	}
	if err := store.Refresh(context.Background(), rec, &s); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	return db, rec
}

// The saving this is for: a session refreshed a minute ago is refreshed again
// on the next page, the next fragment and the next save, and every one of those
// used to send an UPDATE across the network to match no rows.
func TestARecentlyRefreshedSessionIssuesNoStatement(t *testing.T) {
	db, rec := refreshOnce(t, time.Now().Add(-time.Minute))

	if db.execs != 0 {
		t.Errorf("Refresh ran %d statements for a session refreshed a minute ago, want 0", db.execs)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("Refresh re-issued %d cookies without writing the row", len(cookies))
	}
}

// And the case the skip must not swallow: past the interval, the statement runs
// and the cookie is re-issued to match the row it just moved.
func TestASessionPastTheIntervalIsWrittenBack(t *testing.T) {
	db, rec := refreshOnce(t, time.Now().Add(-2*refreshInterval))

	if db.execs != 1 {
		t.Errorf("Refresh ran %d statements for a stale session, want 1", db.execs)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 1 {
		t.Fatalf("Refresh set %d cookies after writing the row, want 1", len(cookies))
	}
}

// The skip is measured from the row and not from process start, so a session
// whose refreshed_at is the zero time -- which is what a row written before the
// column had a default would read as -- is written rather than skipped forever.
func TestAZeroRefreshedAtIsWritten(t *testing.T) {
	db, _ := refreshOnce(t, time.Time{})

	if db.execs != 1 {
		t.Errorf("Refresh ran %d statements for a session with no refreshed_at, want 1", db.execs)
	}
}
