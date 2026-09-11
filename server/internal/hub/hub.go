




package hub

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)




var errGone = errors.New("hub: the room unloaded")

const (
	
	
	
	
	inboxSize = 64

	
	
	
	storeTimeout = 2 * time.Second

	
	
	writeDeadline = 5 * time.Second
)



type Options struct {
	
	
	
	Store Store

	
	
	Version string

	
	
	
	SnapshotInterval time.Duration

	
	
	
	UnloadGrace time.Duration

	
	
	DragInterval time.Duration

	
	
	SendBuffer int

	
	
	ReadLimit int64

	
	
	Rate  int
	Burst int
	Overs int

	
	OverWindow time.Duration

	
	
	
	
	
	WriteHP func(ctx context.Context, character ulid.ULID, hp int) error

	
	
	
	
	
	ConnsPerUser int
	ConnsPerRoom int
}

func (o Options) withDefaults() Options {
	if o.SnapshotInterval <= 0 {
		o.SnapshotInterval = 5 * time.Second
	}
	if o.UnloadGrace <= 0 {
		o.UnloadGrace = 10 * time.Minute
	}
	if o.DragInterval <= 0 {
		o.DragInterval = 50 * time.Millisecond
	}
	if o.SendBuffer <= 0 {
		o.SendBuffer = 256
	}
	if o.ReadLimit <= 0 {
		o.ReadLimit = 64 << 10
	}
	if o.Rate <= 0 {
		o.Rate = 60
	}
	if o.Burst <= 0 {
		o.Burst = 120
	}
	if o.Overs <= 0 {
		o.Overs = 5
	}
	if o.OverWindow <= 0 {
		o.OverWindow = time.Minute
	}
	if o.ConnsPerUser <= 0 {
		o.ConnsPerUser = 4
	}
	if o.ConnsPerRoom <= 0 {
		o.ConnsPerRoom = 64
	}
	if o.Version == "" {
		o.Version = Version()
	}

	return o
}










type Hub struct {
	store   Store
	queries *queries.Queries
	opts    Options
	version string
	writeHP func(ctx context.Context, character ulid.ULID, hp int) error

	mu     sync.Mutex
	rooms  map[ulid.ULID]*actor
	closed bool
}



func New(q *queries.Queries, opts Options) *Hub {
	opts = opts.withDefaults()
	if opts.Store == nil {
		opts.Store = NewStore(q)
	}
	if opts.WriteHP == nil && q != nil {
		opts.WriteHP = sheetHP(q)
	}

	return &Hub{
		store:   opts.Store,
		queries: q,
		opts:    opts,
		version: opts.Version,
		writeHP: opts.WriteHP,
		rooms:   make(map[ulid.ULID]*actor),
	}
}



func (h *Hub) Version() string { return h.version }












func (h *Hub) Dispatch(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd room.Command) error {
	if err := h.resolve(ctx, roomID, who, cmd); err != nil {
		return err
	}

	
	
	
	
	for range 2 {
		a, err := h.room(ctx, roomID)
		if err != nil {
			return err
		}

		reply := make(chan error, 1)
		if err := a.post(ctx, dispatch{who: who, cmd: cmd, reply: reply}); err != nil {
			if errors.Is(err, errGone) {
				continue
			}

			return err
		}

		select {
		case err := <-reply:
			if errors.Is(err, errGone) {
				continue
			}

			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errGone
}












func (h *Hub) Notify(roomID ulid.ULID, cmd room.Command) {
	h.mu.Lock()
	a, ok := h.rooms[roomID]
	h.mu.Unlock()
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()

	reply := make(chan error, 1)
	_ = a.post(ctx, dispatch{cmd: cmd, reply: reply})
}







func (h *Hub) Close(ctx context.Context, roomID ulid.ULID) {
	h.mu.Lock()
	a, ok := h.rooms[roomID]
	h.mu.Unlock()
	if !ok {
		return
	}

	done := make(chan struct{})
	if err := a.post(ctx, closeRoom{done: done}); err != nil {
		return
	}

	select {
	case <-done:
	case <-ctx.Done():
	}
}


func (h *Hub) find(roomID ulid.ULID) *actor {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.rooms[roomID]
}
















func view[T any](ctx context.Context, h *Hub, roomID ulid.ULID, load bool, fn func(*actor) *T) (*T, bool) {
	var a *actor
	if load {
		loaded, err := h.room(ctx, roomID)
		if err != nil {
			return nil, false
		}
		a = loaded
	} else if a = h.find(roomID); a == nil {
		return nil, false
	}

	reply := make(chan any, 1)
	if err := a.post(ctx, ask{fn: func(a *actor) any { return fn(a) }, reply: reply}); err != nil {
		return nil, false
	}

	select {
	case answer := <-reply:
		value, _ := answer.(*T)

		return value, value != nil
	case <-ctx.Done():
		return nil, false
	}
}





func (h *Hub) Players(ctx context.Context, roomID ulid.ULID) ([]room.Player, bool) {
	players, ok := view(ctx, h, roomID, false, func(a *actor) *[]room.Player {
		p := a.players()

		return &p
	})
	if !ok {
		return nil, false
	}

	return *players, true
}






type TableView struct {
	Table room.Table
	Pawns map[ulid.ULID]int
}












func (h *Hub) Table(ctx context.Context, roomID ulid.ULID) (*TableView, bool) {
	return view(ctx, h, roomID, true, (*actor).table)
}
















func (h *Hub) Pawn(ctx context.Context, roomID ulid.ULID, pawnID ulid.ULID, role room.Role) (*room.Pawn, bool) {
	return view(ctx, h, roomID, false, func(a *actor) *room.Pawn { return a.pawn(pawnID, role) })
}














type InitiativeView struct {
	Initiative room.Initiative
	Pawns      map[ulid.ULID]room.Pawn
	Table      room.Table
}












func (h *Hub) Initiative(ctx context.Context, roomID ulid.ULID, role room.Role) (*InitiativeView, bool) {
	return view(ctx, h, roomID, true, func(a *actor) *InitiativeView { return a.initiative(role) })
}



func (h *Hub) spawn(ctx context.Context, roomID ulid.ULID) (*SpawnView, bool) {
	return view(ctx, h, roomID, true, (*actor).spawn)
}












func (h *Hub) Shutdown(ctx context.Context) {
	h.mu.Lock()
	h.closed = true
	rooms := make([]*actor, 0, len(h.rooms))
	for _, a := range h.rooms {
		rooms = append(rooms, a)
	}
	h.mu.Unlock()

	var wg sync.WaitGroup
	for _, a := range rooms {
		wg.Add(1)
		go func() {
			defer wg.Done()

			postCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
			defer cancel()

			done := make(chan struct{})
			if err := a.post(postCtx, shutdown{done: done}); err != nil {
				return
			}
			select {
			case <-done:
			case <-ctx.Done():
			}
		}()
	}
	wg.Wait()

	h.mu.Lock()
	clear(h.rooms)
	h.mu.Unlock()
}







func (h *Hub) room(ctx context.Context, roomID ulid.ULID) (*actor, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()

		return nil, ErrNoRoom
	}
	if a, ok := h.rooms[roomID]; ok {
		h.mu.Unlock()

		return a, nil
	}
	h.mu.Unlock()

	loaded, err := h.store.Load(ctx, roomID)
	if err != nil {
		return nil, err
	}
	state, failed := hydrate(roomID, loaded)
	if failed {
		h.preserve(roomID, loaded.Snapshot)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil, ErrNoRoom
	}
	if a, ok := h.rooms[roomID]; ok {
		return a, nil
	}

	a := newActor(h, roomID, state, state.Seq)
	h.rooms[roomID] = a
	go a.run()

	return a, nil
}









func (h *Hub) preserve(roomID ulid.ULID, snapshot []byte) {
	store := h.store
	kept := append([]byte(nil), snapshot...)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()

		if err := store.Preserve(ctx, roomID, kept); err != nil {
			slog.Error("Failed to keep an unreadable snapshot", "room", roomID, "error", err)
		}
	}()
}






func (h *Hub) retire(a *actor) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(a.inbox) > 0 || len(a.conns) > 0 {
		return false
	}
	if h.rooms[a.id] != a {
		return false
	}

	delete(h.rooms, a.id)
	close(a.done)

	return true
}





func (h *Hub) forget(a *actor) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[a.id] == a {
		delete(h.rooms, a.id)
	}

	
	
	
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}



func (h *Hub) live(roomID ulid.ULID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.rooms[roomID]

	return ok
}


func (a *actor) post(ctx context.Context, m any) error {
	select {
	case <-a.done:
		return errGone
	default:
	}

	select {
	case a.inbox <- m:
		return nil
	case <-a.done:
		return errGone
	case <-ctx.Done():
		return ctx.Err()
	}
}








func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, roomID ulid.ULID, p room.Player, member Membership) {
	ctx := r.Context()

	a, err := h.room(ctx, roomID)
	if err != nil {
		http.NotFound(w, r)

		return
	}

	h.attach(w, r, a, p, member)
}
