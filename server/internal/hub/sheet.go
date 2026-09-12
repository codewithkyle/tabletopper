package hub

import (
	"context"
	"log/slog"
	"sync"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

type sheetWriter struct {
	write   func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error
	mu      sync.Mutex
	pending map[ulid.ULID]room.SheetVitals
	wake    chan struct{}
	quit    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

func newSheetWriter(write func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error) *sheetWriter {
	if write == nil {
		return nil
	}
	w := &sheetWriter{
		write:   write,
		pending: make(map[ulid.ULID]room.SheetVitals),
		wake:    make(chan struct{}, 1),
		quit:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go w.run()
	return w
}
func (w *sheetWriter) put(character ulid.ULID, v room.SheetVitals) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.pending[character] = v
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}
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
func (w *sheetWriter) flush() {
	for {
		character, v, ok := w.next()
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		err := w.write(ctx, character, v)
		cancel()
		if err != nil {
			slog.Error("Failed to write a pawn's vitals back to its sheet", "character", character, "error", err)
		}
	}
}
func (w *sheetWriter) next() (ulid.ULID, room.SheetVitals, bool) {
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
		return ulid.ULID{}, room.SheetVitals{}, false
	}
	v := w.pending[lowest]
	delete(w.pending, lowest)
	return lowest, v, true
}
