package hub

import (
	"context"
	"log/slog"
	"sync"

	"github.com/oklog/ulid/v2"
)

type sheetWriter struct {
	write   func(ctx context.Context, character ulid.ULID, hp int) error
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
