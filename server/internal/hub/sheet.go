package hub

import (
	"context"
	"log/slog"
	"sync"

	"github.com/oklog/ulid/v2"
)

// sheetWriter puts hit points back on character sheets, from off the room's
// goroutine, one statement at a time.
//
// LATEST VALUE PER CHARACTER, IN FLIGHT ONE AT A TIME. The room used to start
// a goroutine per update with nothing between them, and two edits fifty
// milliseconds apart could commit in either order -- so the sheet could end up
// holding the number the table had just corrected. Here the room puts a value
// in; the writer takes the newest value for each character and writes it, and
// a value put in while a write is in flight replaces the one waiting rather
// than queueing behind it. It is the drag-coalescing shape, applied to writes.
//
// A NIL WRITER IS A WRITER THAT WRITES NOTHING, which is what a hub with no
// database gets, so the room can call it without asking.
type sheetWriter struct {
	write func(ctx context.Context, character ulid.ULID, hp int) error

	mu      sync.Mutex
	pending map[ulid.ULID]int

	wake    chan struct{}
	quit    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

func newSheetWriter(write func(ctx context.Context, character ulid.ULID, hp int) error) *sheetWriter {
	if write == nil {
		return nil
	}

	w := &sheetWriter{
		write:   write,
		pending: make(map[ulid.ULID]int),
		wake:    make(chan struct{}, 1),
		quit:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go w.run()

	return w
}

// put records the value a character's sheet should hold. It never blocks.
func (w *sheetWriter) put(character ulid.ULID, hp int) {
	if w == nil {
		return
	}

	w.mu.Lock()
	w.pending[character] = hp
	w.mu.Unlock()

	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// stop writes whatever is still owed and returns when it has. It is bounded by
// the statement's own deadline, so a room ending on a stalled database is
// held up by one storeTimeout and not for ever.
func (w *sheetWriter) stop() {
	if w == nil {
		return
	}

	w.once.Do(func() { close(w.quit) })
	<-w.stopped
}

func (w *sheetWriter) run() {
	defer close(w.stopped)

	for {
		select {
		case <-w.quit:
			w.flush()

			return
		case <-w.wake:
			w.flush()
		}
	}
}

// flush writes every pending value, lowest character id first so that a
// burst lands in the same order twice.
func (w *sheetWriter) flush() {
	for {
		character, hp, ok := w.next()
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		err := w.write(ctx, character, hp)
		cancel()
		if err != nil {
			slog.Error("Failed to write a pawn's hit points back to its sheet", "character", character, "error", err)
		}
	}
}

func (w *sheetWriter) next() (ulid.ULID, int, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var lowest ulid.ULID
	found := false
	for id := range w.pending {
		if !found || id.Compare(lowest) < 0 {
			lowest, found = id, true
		}
	}
	if !found {
		return ulid.ULID{}, 0, false
	}

	hp := w.pending[lowest]
	delete(w.pending, lowest)

	return lowest, hp, true
}
