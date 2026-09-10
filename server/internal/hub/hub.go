// Package hub puts the room protocol on a socket. internal/room is pure: it
// knows commands, events, state and the two audiences, and it knows nothing
// about goroutines, connections, time or the database. This package is all four
// of those and nothing else -- every decision about what a command means still
// belongs on the other side of the import.
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

// errGone is a room that retired between a caller reading the map and posting
// into its inbox. It never reaches a browser: Dispatch answers it by loading
// the room again, which is the same work the first call was going to do.
var errGone = errors.New("hub: the room unloaded")

const (
	// inboxSize is how far a room may fall behind its senders before they
	// block. Every message in it is a command or a lifecycle event, and the
	// actor answers each in microseconds, so this is slack for a burst rather
	// than a queue anything is expected to sit in.
	inboxSize = 64

	// storeTimeout bounds the two statements the room goroutine runs itself.
	// Long enough for a busy database and short enough that a table full of
	// people is not waiting on one that has stopped answering.
	storeTimeout = 2 * time.Second

	// writeDeadline is per frame. A socket that cannot take a few hundred
	// bytes in five seconds is not a socket anybody is playing over.
	writeDeadline = 5 * time.Second
)

// Options are the tunables, all of them, so a test can make a room save in
// milliseconds and unload in tens of them without a clock abstraction anywhere.
type Options struct {
	// Store is where snapshots live. New fills it from the queries it was
	// given when it is left nil, which is what production does; the tests pass
	// one of their own and no database at all.
	Store Store

	// Version is the build string in every snapshot. Left empty, it is
	// whatever Version reports.
	Version string

	// SnapshotInterval is how often a dirty room is written back, and it is
	// also how often a room asks whether it has been empty long enough to
	// unload.
	SnapshotInterval time.Duration

	// UnloadGrace is how long a room with nobody in it stays in memory. A page
	// reload closes a socket and opens another a few hundred milliseconds
	// later, and this is what keeps that from being a database read.
	UnloadGrace time.Duration

	// DragInterval is the drag coalescing tick. Twenty a second looks smooth
	// and bounds what a fast mouse costs everybody else.
	DragInterval time.Duration

	// SendBuffer is how many frames may be queued for one connection before it
	// is dropped as too slow. See actor.send.
	SendBuffer int

	// ReadLimit caps one inbound frame. Strokes are chunked to stay far under
	// it; nothing else in the protocol comes close.
	ReadLimit int64

	// Rate and Burst are the per-connection token bucket, in commands per
	// second. Overs is how many refusals inside OverWindow end the connection.
	Rate  int
	Burst int
	Overs int

	// OverWindow is the span the over count is measured across.
	OverWindow time.Duration

	// WriteHP is the one statement the room runs against a character sheet:
	// a player pawn's hit points, written back so the sheet says what
	// happened at the table. New fills it from the queries when it is left
	// nil, as it does Store; a test passes one of its own, and a test that
	// passes nothing gets a room that writes no sheets.
	WriteHP func(ctx context.Context, character ulid.ULID, hp int) error

	// ConnsPerUser and ConnsPerRoom cap open sockets. Every join costs the
	// room goroutine an apply, a broadcast and a whole projected snapshot, and
	// the rate limit is per socket -- so without these one person with a
	// script is N sockets multiplying both. A GM with three tabs is the most
	// anybody honest opens; sixty-four is more people than any table seats.
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

// Hub is the live rooms in this process and the way in to each of them. It is
// the only thing in the design that is shared between goroutines, and what it
// shares is a map of pointers -- never a room's state, which belongs to one
// goroutine from the moment it is loaded until it unloads.
//
// THERE IS NO HORIZONTAL SCALING BEHIND THIS AND THE DESIGN ASSUMES SO. Room
// state lives in one process. If that ever has to change the answer is sticky
// routing by room, not a shared store, and this type is where the routing would
// go.
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

// New builds the hub. q is what the production Store and the command resolution
// read rows through; a test passes nil and a Store of its own.
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

// Version is the build string this hub stamps into every snapshot, and the one
// the room page puts on its bundle URL so the two cannot disagree.
func (h *Hub) Version() string { return h.version }

// Dispatch runs one command against a room and waits for the answer, loading
// the room if it is not live.
//
// IT LOADS RATHER THAN NO-OPPING, unlike Notify, because the caller is a person
// clicking a control and expecting it to have happened. A GM adjusting the grid
// from a page whose socket has not connected yet would otherwise see the form
// succeed and the change vanish.
//
// The error it returns is the protocol's own *room.Error where the command was
// refused, which is what lets a handler put a heading and a message in the
// alert modal without knowing anything about the command it sent.
func (h *Hub) Dispatch(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd room.Command) error {
	if err := h.resolve(ctx, roomID, who, cmd); err != nil {
		return err
	}

	// Twice, and no more: the only way the first attempt fails is that the
	// room retired in the gap between being found and being written to, and
	// the second attempt loads a fresh one that cannot retire again while a
	// message is on its way in.
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

// Notify mirrors a fact the rooms row already holds into a live room, and does
// nothing at all when the room is not live.
//
// THAT IS THE WHOLE DIFFERENCE FROM Dispatch AND IT IS CORRECT. The row is the
// writer of record for the name, the lock and the membership, so a room that is
// not running has nothing to be told -- the next load reads all three off the
// row. Loading a room to inform it of something it will read anyway would put a
// snapshot's worth of work behind a lock toggle.
//
// The events it produces carry no `by`. Nobody sent them over a socket: an HTTP
// route wrote a row and this is the room catching up with it.
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

// Close ends a room: everybody in it is told, every connection is dropped, and
// the room saves one last time before it unloads.
//
// THE SNAPSHOT IS KEPT ON PURPOSE. Closing is not deleting -- the row stays and
// the GM can reopen it -- and a room that came back empty would make "close"
// mean "throw the table away", which is what Delete is for.
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

// find is a live room or nil, and it never loads one.
func (h *Hub) find(roomID ulid.ULID) *actor {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.rooms[roomID]
}

// view asks a room one question and waits for the answer, which is a copy the
// caller may keep: it was made on the room's goroutine and nothing else holds
// it. ok is false when the room is not there to ask, when it went away between
// being found and answering, or when the caller's context ran out first.
//
// load IS THE ONE THING THE CALLERS DIFFER ON, and each of them says why on
// its own doorstep. A passive read -- the player list, one pawn -- answers
// false for a room that is not running, because booting a room to draw a
// panel would let a stale tab start a room nobody is in. A read that is one
// click from a Dispatch -- the layer manager, the tracker -- loads, because
// the Dispatch was going to.
//
// EVERYTHING BELOW THIS IS A CLOSURE OVER ONE OF THE ACTOR'S OWN METHODS. The
// message type is the same for all of them, so the room's drain answers every
// one of them the same way and a view cannot be forgotten there.
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

// Players is the live player list for the members fragment, and ok is false
// when the room is not running -- which is the caller's cue to fall back to the
// session rows. It does not load the room: a page load must not start a room
// nobody is in.
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

// TableView is the room's table plus how many pawns stand on each layer. It is
// the answer to the two questions the GM's configuration dialogs ask, and they
// are answered together because they are read together: a layer manager that
// fetched the table and then counted the pawns would be quoting a number from
// one instant and offering to delete from another.
type TableView struct {
	Table room.Table
	Pawns map[ulid.ULID]int
}

// Table is the live table for the layer manager and the grid form.
//
// IT LOADS THE ROOM RATHER THAN ANSWERING FALSE, which is the opposite of what
// Players does, and the difference is what the caller is about to do.
// Somebody opening the layer manager is one click away from Dispatch, which
// loads the room anyway; making them wait for the fragment to be wrong first
// buys nothing.
//
// The false it can still answer is a hub that is shutting down or a room whose
// snapshot will not load, which is the caller's cue to say so rather than to
// draw an empty table.
func (h *Hub) Table(ctx context.Context, roomID ulid.ULID) (*TableView, bool) {
	return view(ctx, h, roomID, true, (*actor).table)
}

// Pawn is one pawn as the asking role may see it, for the fragments that draw
// a pawn's panel and its stat block. It answers nil for a pawn that is not
// there, for a room that is not running, and -- the case that matters -- for
// one this role is shown nothing of.
//
// THE ROLE IS A PARAMETER AND THE PROJECTION IS NOT OPTIONAL. See
// room.ProjectedPawn: there is no accessor here that hands back the stored
// pawn, because a fragment route is a second door into state the socket
// projects on the way out, and a door with no lock on it is how a player reads
// a hidden monster's hit points.
//
// IT DOES NOT LOAD THE ROOM, which is Players' rule rather than Table's. Both
// callers are drawing a panel about a pawn that is on somebody's screen, so the
// room is running by definition; loading one to answer would mean a stale
// window in a reloaded tab could start a room that nobody is in.
func (h *Hub) Pawn(ctx context.Context, roomID ulid.ULID, pawnID ulid.ULID, role room.Role) (*room.Pawn, bool) {
	return view(ctx, h, roomID, false, func(a *actor) *room.Pawn { return a.pawn(pawnID, role) })
}

// InitiativeView is the turn tracker as one role may see it, the pawns it
// names, and the table those pawns were projected against.
//
// THE THREE ARE ANSWERED TOGETHER BECAUSE THE STRIP DRAWS THEM TOGETHER. A card
// is a pawn's portrait, its name and its wounds under a line of the tracker, so
// a fragment that fetched the tracker and then asked for each pawn separately
// would be quoting a turn order from one instant and a goblin's hit points from
// another -- and would be applying a second, different filter on the way. See
// room.ProjectedInitiative, which is where that second filter would have been
// the bug.
//
// PAWNS IS KEYED BY ID because the caller looks up one pawn per member of every
// line, and a slice would be a scan per pip.
type InitiativeView struct {
	Initiative room.Initiative
	Pawns      map[ulid.ULID]room.Pawn
	Table      room.Table
}

// Initiative is the live turn tracker for the strip over the table.
//
// IT LOADS THE ROOM, which is Table's rule rather than Pawn's. The strip is
// fetched on every page load, by everybody, and the socket connects a moment
// later and would have loaded the room anyway; making the first fetch answer
// with an empty tracker so that the second one could be right buys nothing.
//
// THE ROLE IS A PARAMETER AND THE PROJECTION IS NOT OPTIONAL, for the reason
// spelled out on Pawn above: a fragment route is a second door into state the
// socket projects on the way out, and a hidden monster must not come back
// through it as a line in somebody's turn order.
func (h *Hub) Initiative(ctx context.Context, roomID ulid.ULID, role room.Role) (*InitiativeView, bool) {
	return view(ctx, h, roomID, true, func(a *actor) *InitiativeView { return a.initiative(role) })
}

// spawn is what resolving a spawn needs out of the running room. It is
// unexported because nothing outside this package resolves a command.
func (h *Hub) spawn(ctx context.Context, roomID ulid.ULID) (*SpawnView, bool) {
	return view(ctx, h, roomID, true, (*actor).spawn)
}

// Shutdown snapshots every live room and closes every connection with
// going-away, which is what turns a deploy into a reconnect rather than an
// error. Every room gets the same deadline, and one that misses it loses only
// what changed since its last save.
//
// THE MESSAGE IS POSTED WITH A CONTEXT THAT CANNOT ALREADY BE DONE, and only
// the wait is bounded by the caller's. post selects between the inbox and the
// context, Go picks between ready cases at random, and a caller whose deadline
// had already passed -- the HTTP drain having spent the shared budget -- would
// have had roughly half its rooms never told to save at all. This is the path
// the whole snapshot design exists for, so the telling is unconditional.
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

// room finds a live room or loads one.
//
// THE LOAD HAPPENS OUTSIDE THE LOCK, which means two joins arriving together
// can both read the row. One of them wins the second check and the other throws
// its state away, which costs a query nobody notices; holding the lock across
// the read instead would put every other room's lookup behind one slow SELECT.
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

// preserve keeps a snapshot hydrate could not read, off the calling goroutine
// and before the room it belongs to can save over it.
//
// BEFORE, BECAUSE THE FIRST SAVE IS AT LEAST ONE INTERVAL AWAY and this is one
// statement with the same deadline. It is still a race on paper; it is not one
// in practice, and a failure here is a log line rather than a refusal to load
// the room, because a table that cannot be opened at all is worse than one
// that came back empty.
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

// retire is a room asking permission to stop, called from its own goroutine.
// The lock is what makes the answer safe: a caller that is about to post has
// either already got its message into the inbox, which the length check sees,
// or is still holding the lock out and will find done closed when it gets
// there.
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

// forget is retire without the conditions, for the two endings a room does not
// get to decline: shutdown and close. Both have already told every connection
// and written the snapshot by the time this runs, so what is left is to take
// the room out of the map and wake anybody who was about to send it something.
func (h *Hub) forget(a *actor) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[a.id] == a {
		delete(h.rooms, a.id)
	}

	// retire cannot have closed this already -- both run on the room's own
	// goroutine and either one ends it -- but a second close would panic and
	// the guard costs nothing.
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

// live reports whether a room is loaded. It exists for the tests, which
// otherwise have to reach into the map they are not supposed to know about.
func (h *Hub) live(roomID ulid.ULID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.rooms[roomID]

	return ok
}

// post sends one message into a room's inbox, or reports that the room is gone.
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

// Serve upgrades one request to a WebSocket and runs it until either side ends
// it. It answers the response itself, including the refusal when the room
// cannot be loaded, so the caller has nothing left to write.
//
// member is re-run on a timer for the life of the socket and a false answer
// closes it; see Membership. The caller has already run it once, which is how
// it knows p is a member at all.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, roomID ulid.ULID, p room.Player, member Membership) {
	ctx := r.Context()

	a, err := h.room(ctx, roomID)
	if err != nil {
		http.NotFound(w, r)

		return
	}

	h.attach(w, r, a, p, member)
}
