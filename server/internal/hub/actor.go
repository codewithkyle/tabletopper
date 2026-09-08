package hub

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

// THE ROOM GOROUTINE. One of these owns one room's state and is the only code
// that reads or writes it, which is what buys the whole design its absence of
// locks: there is no shared access to lock against. Everything that wants
// something from a room -- a socket delivering a command, an HTTP handler
// changing a setting, the fragment drawing the player list, shutdown -- sends
// it a message and waits or does not.
//
// WHAT IT OWNS: the state, the set of live connections, the two sequence
// counters, the dirty flag, the pending drags and its own id source. None of
// those has a mutex anywhere near it and none of them may be read from another
// goroutine.

// message is one thing sent into a room's inbox. They are separate types rather
// than one struct with a kind, so the fields each one needs are the fields it
// has and a handler cannot read a field the sender never set.
type (
	// join seats a connection. The client is not in the room's set yet: the
	// actor adds it between the join event and the snapshot, so that the
	// joiner never receives an event that its own snapshot already contains.
	join struct{ c *client }

	// leave drops one connection. The player row stays either way -- losing
	// wifi mid-fight must not delete somebody's pawns -- and only the last
	// connection a user has in the room flips them to disconnected.
	leave struct{ c *client }

	// command is one frame off a socket. err is set when the frame could not
	// be decoded or the sender is over its rate limit, in which case cmd is
	// nil and the actor's whole job is to answer with the error: stamping a
	// sequence number is the actor's, so the frame has to be built here even
	// when nothing about the room changed.
	command struct {
		c   *client
		cmd room.Command
		cid string
		err error
	}

	// dispatch is a command from an HTTP handler, which waits for the answer
	// so it can turn a refusal into the alert modal.
	dispatch struct {
		who   room.Actor
		cmd   room.Command
		reply chan error
	}

	// roster is the player list, for the members fragment.
	roster struct{ reply chan []room.Player }

	// tableView is the table and the pawn count per layer, for the layer
	// manager and the grid form.
	tableView struct{ reply chan *TableView }

	// shutdown snapshots and closes every connection with going-away, so the
	// browsers reconnect to the next process rather than showing an error.
	shutdown struct{ done chan struct{} }

	// closeRoom ends the session: everybody is told, everybody is dropped, and
	// the room saves one last time so reopening it comes back to the pawns
	// where they were left.
	closeRoom struct{ done chan struct{} }
)

type actor struct {
	hub   *Hub
	id    ulid.ULID
	inbox chan any

	// done is closed when this actor has retired and removed itself from the
	// hub's map. Every send into inbox selects on it, so a caller holding a
	// pointer to a room that unloaded is told to ask for the room again rather
	// than blocking on an inbox nobody is reading.
	done chan struct{}

	state *room.State
	conns map[*client]struct{}

	// THE TWO SEQUENCE COUNTERS, ONE PER AUDIENCE, and the reason there are
	// two is the reason a gap is worth detecting at all.
	//
	// A single room-wide counter would number the GM-only events as well, so
	// every player's stream would have holes in it by design and "a gap means
	// resync" could never be the rule. Per connection would mean encoding each
	// frame once per client rather than once per audience, which is the cost
	// the two-audience projection was built to avoid.
	//
	// Per audience works because of a property of the catalog: every event
	// that is not transient goes to a WHOLE audience -- ToAll, ToGM or
	// ToPlayers -- and the three partial audiences carry only transient events.
	// So each of these numbers every state event its audience receives, with
	// nothing missing, and a client that sees seq jump knows the server
	// dropped something rather than that it was not addressed.
	//
	// TRANSIENT EVENTS DO NOT ADVANCE THEM. A ping, a drag ghost, an error and
	// a kick carry the current value and leave it where it is, so the client's
	// check is over exactly the events its reducer runs.
	seqGM, seqPlayer uint64

	dirty bool

	// drags is the coalescing buffer: at most one pending drag per anchor
	// pawn, flushed on a timer. A fast mouse and a slow one then cost the same
	// bandwidth, which is what keeps the one hot path in the protocol bounded.
	drags     map[ulid.ULID]command
	dragTimer *time.Timer

	// emptySince is when the last connection left, and the zero value means
	// somebody is still here. A room with nobody in it stays loaded for the
	// grace period so that a refresh or a flaky connection is instant.
	emptySince time.Time

	entropy *ulid.MonotonicEntropy
}

func newActor(h *Hub, id ulid.ULID, state *room.State, seq uint64) *actor {
	timer := time.NewTimer(time.Hour)
	timer.Stop()

	return &actor{
		hub:       h,
		id:        id,
		inbox:     make(chan any, inboxSize),
		done:      make(chan struct{}),
		state:     state,
		conns:     make(map[*client]struct{}),
		seqGM:     seq,
		seqPlayer: seq,
		drags:     make(map[ulid.ULID]command),
		dragTimer: timer,

		// EMPTY FROM THE MOMENT IT LOADS, because a room can be loaded by
		// something that never connects: a GM toggling the lock from a page
		// whose socket is not open dispatches into a room that then has nobody
		// in it. Starting this at the zero value would leave that room in
		// memory for the life of the process, since nothing would ever set it.
		emptySince: time.Now(),

		// Monotonic within a millisecond, so several pawns spawned in one
		// operation sort in the order they were created rather than in
		// whichever order the random bytes fell.
		entropy: ulid.Monotonic(rand.Reader, 0),
	}
}

// env is what an Apply is given that is neither state nor command.
func (a *actor) env() room.Env {
	return room.Env{
		NewID:   func() ulid.ULID { return ulid.MustNew(ulid.Now(), a.entropy) },
		Version: a.hub.version,
	}
}

// run is the goroutine. Every case in it either handles a message or fires a
// timer, and nothing else in the process touches what it owns.
func (a *actor) run() {
	save := time.NewTicker(a.hub.opts.SnapshotInterval)
	defer save.Stop()
	defer a.dragTimer.Stop()

	for {
		select {
		case m := <-a.inbox:
			if a.handle(m) {
				return
			}

		case <-a.dragTimer.C:
			a.flushDrags()

		case <-save.C:
			// The same tick asks both questions, which is why there is one
			// timer rather than two: a room that has been empty for the grace
			// period has just been written back by the line above it, so
			// unloading costs nothing more.
			a.save()
			if a.idle() && a.hub.retire(a) {
				a.drain()

				return
			}
		}
	}
}

// handle answers one message and reports whether the room is finished.
func (a *actor) handle(m any) bool {
	switch m := m.(type) {
	case join:
		a.join(m.c)

	case leave:
		a.leave(m.c)

	case command:
		a.fromClient(m)

	case dispatch:
		m.reply <- a.exec(m.who, m.cmd, nil, "")

	case roster:
		m.reply <- a.players()

	case tableView:
		m.reply <- a.table()

	case shutdown:
		a.stopAll(closeGoingAway, reasonRestarting)
		a.save()
		a.hub.forget(a)
		a.drain()
		close(m.done)

		return true

	case closeRoom:
		a.exec(room.Actor{}, &room.RoomClose{}, nil, "")
		a.stopAll(closeNormal, reasonClosed)
		a.save()
		a.hub.forget(a)
		a.drain()
		close(m.done)

		return true
	}

	return false
}

// join is the ordering rule the whole reconnect story rests on.
//
// THE JOINER MUST NEVER SEE AN EVENT ITS SNAPSHOT ALREADY CONTAINS. So the join
// is applied and broadcast while the new connection is still outside the room's
// set -- the people already here learn about the arrival -- and only then is it
// added and sent the snapshot, which therefore carries the sequence number that
// join event was given. Everything after it is strictly newer.
func (a *actor) join(c *client) {
	a.emptySince = time.Time{}

	a.exec(c.who, &room.PlayerJoin{Player: c.player}, nil, "")
	a.conns[c] = struct{}{}
	a.snapshot(c)
}

// leave drops a connection and, when it was the person's last one in the room,
// marks them disconnected.
//
// THE ROW IS NOT REMOVED. A disconnect is one field changing; leaving for good
// is the Leave button, which is an HTTP route and arrives as player.leave.
func (a *actor) leave(c *client) {
	if _, ok := a.conns[c]; !ok {
		return
	}
	delete(a.conns, c)

	if !a.connected(c.who.ID) {
		a.exec(room.Actor{}, &room.PlayerSetConnected{ID: c.who.ID, Connected: false}, nil, "")
	}
	if len(a.conns) == 0 {
		a.emptySince = time.Now()
	}
}

// fromClient is one frame off a socket, which is three cases: a frame that
// could not be read, a drag that is held for the next flush, and everything
// else, which is applied now.
func (a *actor) fromClient(m command) {
	if m.err != nil {
		a.refuse(m.c, m.cid, m.err)

		return
	}

	// A DRAG IS NOT APPLIED WHEN IT ARRIVES. The latest one per anchor is kept
	// and the timer sends it, so twenty updates a second is what a drag costs
	// however fast the mouse is moving. The previous pending drag for the same
	// anchor is simply overwritten: nobody wants the position the pointer was
	// at thirty milliseconds ago.
	if drag, ok := m.cmd.(*room.PawnDrag); ok {
		if len(a.drags) == 0 {
			a.dragTimer.Reset(a.hub.opts.DragInterval)
		}
		a.drags[drag.Anchor] = m

		return
	}

	if err := a.exec(m.c.who, m.cmd, m.c, m.cid); err != nil {
		a.refuse(m.c, m.cid, err)
	}
}

// flushDrags sends one dragging event per anchor that moved since the last
// flush, in id order so that a room with several drags in flight behaves the
// same way twice.
func (a *actor) flushDrags() {
	anchors := make([]ulid.ULID, 0, len(a.drags))
	for id := range a.drags {
		anchors = append(anchors, id)
	}
	slices.SortFunc(anchors, func(x, y ulid.ULID) int { return x.Compare(y) })

	for _, id := range anchors {
		m := a.drags[id]
		delete(a.drags, id)

		// A refusal is answered like any other: the pawn may have been removed
		// or hidden between the drag starting and this flush, and the client
		// that is drawing a ghost of it needs to be told to stop.
		if err := a.exec(m.c.who, m.cmd, m.c, m.cid); err != nil {
			a.refuse(m.c, m.cid, err)
		}
	}
}

// exec is authorize, apply, emit -- the only path by which the state changes.
func (a *actor) exec(who room.Actor, cmd room.Command, sender *client, cid string) error {
	if err := cmd.Authorize(a.state, who); err != nil {
		return err
	}

	ems, err := cmd.Apply(a.state, who, a.env())
	if err != nil {
		return err
	}

	a.emit(ems, who, sender, nil)

	return nil
}

// emit stamps, encodes and fans out, once per audience rather than once per
// connection -- which is the whole point of projecting server-side, and the
// reason the sequence counters are per audience too.
//
// only, when set, narrows every emission to that one connection. The join
// snapshot is the only caller that uses it, because a second tab belonging to
// the same person has no business receiving a snapshot it did not ask for.
func (a *actor) emit(ems []room.Emission, who room.Actor, sender, only *client) {
	var by *ulid.ULID
	if !who.ID.IsZero() {
		id := who.ID
		by = &id
	}

	for _, em := range ems {
		counted := counts(em.Event)

		// AN EVENT THAT COUNTS MUST REACH A WHOLE AUDIENCE. Everything else
		// would leave the clients in that audience who were not addressed with
		// a hole in their sequence, which is exactly the signal they treat as
		// "the server dropped something". Nothing in the catalog does this
		// today; if something starts to, the clients recover by resyncing and
		// this line says why they suddenly are.
		if counted && (em.To == room.ToSender || em.To == room.ToOthers || em.To == room.ToPlayer) {
			slog.Error("A state event was addressed to part of an audience",
				"room", a.id, "event", frameType(em.Event))
		}

		for _, role := range a.audience(em, who) {
			seq := a.advance(role, counted)

			ev := room.ForRole(em.Event, role)
			if ev == nil {
				continue
			}

			// The snapshot carries the sequence in two places -- the envelope
			// and the state -- and they have to agree, because the client
			// takes its counter from the state it just adopted.
			if snap, ok := ev.(*room.Snapshot); ok {
				snap.State.Seq = seq
			}

			frame, err := room.EncodeEvent(ev, seq, by)
			if err != nil {
				slog.Error("Failed to encode an event", "room", a.id, "event", frameType(ev), "error", err)

				continue
			}

			for _, c := range a.recipients(em, role, who, sender, only) {
				a.send(c, frame)
			}
		}

		if counted {
			a.dirty = true
		}

		// The effect runs between emissions rather than after all of them,
		// which is the order the protocol asks for: a kick tells the person
		// first and closes their sockets, and the announcement that follows is
		// then addressed to the people who are still here.
		a.effect(em)
	}
}

// counts reports whether an event advances its audience's sequence, which is
// the same question as whether a client would notice its absence.
//
// TRANSIENT EVENTS DO NOT: a ping, a drag ghost, an error and a kick are all
// things a client that missed one is none the worse for.
//
// NEITHER DOES THE SNAPSHOT, and that is the less obvious half. A snapshot is
// not a step in the stream -- it is the state the stream continues from, sent
// to one connection because that connection asked. Numbering it would put a
// gap in every other client's sequence every time somebody resynced, which is
// exactly the signal a gap is supposed to be.
func counts(ev room.Event) bool {
	if ev.Transient() {
		return false
	}
	_, snapshot := ev.(*room.Snapshot)

	return !snapshot
}

// audience is which of the two projections an emission has to be encoded for.
// It is deliberately not "which connections are here": the counters belong to
// the room rather than to whoever happens to be connected, so an event with no
// GM watching still advances the GM's sequence and the GM's next snapshot picks
// up where it left off.
func (a *actor) audience(em room.Emission, who room.Actor) []room.Role {
	switch em.To {
	case room.ToAll, room.ToOthers:
		return []room.Role{room.RoleGM, room.RolePlayer}
	case room.ToGM:
		return []room.Role{room.RoleGM}
	case room.ToPlayers:
		return []room.Role{room.RolePlayer}
	case room.ToSender:
		return []room.Role{who.Role}
	case room.ToPlayer:
		// Read off the connections rather than off the player row, because the
		// one event that uses this audience is the kick and the kick has
		// already removed the row by the time it is sent.
		var roles []room.Role
		for c := range a.conns {
			if c.who.ID == em.Player && !slices.Contains(roles, c.who.Role) {
				roles = append(roles, c.who.Role)
			}
		}

		return roles
	}

	return nil
}

// recipients is the connections in one audience that this emission reaches.
func (a *actor) recipients(em room.Emission, role room.Role, who room.Actor, sender, only *client) []*client {
	if only != nil {
		if only.who.Role != role {
			return nil
		}

		return []*client{only}
	}

	var out []*client
	for c := range a.conns {
		if c.who.Role != role {
			continue
		}

		switch em.To {
		case room.ToSender:
			if c.who.ID != who.ID {
				continue
			}
		case room.ToOthers:
			// The connection, not the person. A second tab belonging to the
			// dragging player is another view of the table and wants the ghost
			// like everybody else.
			if c == sender {
				continue
			}
		case room.ToPlayer:
			if c.who.ID != em.Player {
				continue
			}
		}

		out = append(out, c)
	}

	return out
}

// advance returns the sequence number this audience's next frame carries, and
// moves the counter on when the frame is one that counts.
func (a *actor) advance(role room.Role, counted bool) uint64 {
	counter := &a.seqPlayer
	if role == room.RoleGM {
		counter = &a.seqGM
	}
	if counted {
		*counter++
	}

	return *counter
}

// send is the backpressure policy in one place, and it is the whole of it: a
// client that cannot keep up with 256 queued frames is not going to catch up,
// so it is dropped and its reconnect is answered with a fresh snapshot. The
// alternative -- blocking the room on the slowest browser at the table -- is
// how one person's bad wifi stops everybody else's game.
func (a *actor) send(c *client, frame []byte) {
	if c.enqueue(frame) {
		return
	}

	slog.Warn("Dropping a connection that cannot keep up", "room", a.id, "user", c.who.ID)
	c.stop(closePolicy, reasonSlow)
	delete(a.conns, c)
}

// refuse answers one client with the error frame for a refusal, correlated with
// the id off the command that caused it so the client knows which of its
// requests failed.
func (a *actor) refuse(c *client, cid string, err error) {
	if c == nil {
		return
	}

	ev := room.NewErrorEvent(cid, err)
	frame, encodeErr := room.EncodeEvent(ev, a.advance(c.who.Role, false), nil)
	if encodeErr != nil {
		slog.Error("Failed to encode an error", "room", a.id, "error", encodeErr)

		return
	}

	a.send(c, frame)
}

// snapshot sends one connection the whole room, projected for its role. It goes
// through the ordinary sync.request command so that a first connect, a resync
// and a reconnect are the same code producing the same frame.
func (a *actor) snapshot(c *client) {
	ems, err := (&room.SyncRequest{}).Apply(a.state, c.who, a.env())
	if err != nil {
		slog.Error("Failed to build a snapshot", "room", a.id, "error", err)

		return
	}

	a.emit(ems, c.who, c, c)
}

// effect is the one thing the hub does that no Apply can. A kick is a decision
// about the protocol -- who is in the room -- and also a decision about
// sockets, which internal/room has never heard of: the person removed has to be
// disconnected, and the room has to stop being on their session rows so the
// homepage stops offering to take them back to it.
//
// Closing the room is the other half of this and is not here, because it is not
// triggered by an event: Close sends the room a message of its own and unloads
// it, and room.closed is what that message emits on the way out.
func (a *actor) effect(em room.Emission) {
	if _, ok := em.Event.(*room.PlayerKicked); !ok {
		return
	}

	a.forget(em.Player)
	for c := range a.conns {
		if c.who.ID == em.Player {
			c.stop(closePolicy, reasonKicked)
			delete(a.conns, c)
		}
	}
	if len(a.conns) == 0 {
		a.emptySince = time.Now()
	}
}

// forget clears the room off the kicked person's session rows, so the homepage
// stops offering to take them back to a table they were removed from.
//
// IT RUNS OFF THE ROOM'S GOROUTINE. Nothing here needs the room's state and the
// room has a table full of people waiting on it, so the one statement goes to a
// goroutine of its own with a deadline; a failure is a log line, because there
// is nobody left on this path to tell.
func (a *actor) forget(user ulid.ULID) {
	store, id := a.hub.store, a.id

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()

		if err := store.ClearMembership(ctx, id, user); err != nil {
			slog.Error("Failed to clear a kicked player's membership", "room", id, "user", user, "error", err)
		}
	}()
}

// save writes the snapshot when there is something new in it. A failure leaves
// the room dirty, so the next tick tries again rather than losing the change.
func (a *actor) save() {
	if !a.dirty {
		return
	}

	// The state carries its own sequence, which is what a reload restores
	// from; the column beside it is for whoever is reading the table by hand.
	a.state.Seq = a.seqGM
	blob, err := room.Marshal(a.state)
	if err != nil {
		slog.Error("Failed to encode a snapshot", "room", a.id, "error", err)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()

	if err := a.hub.store.Save(ctx, a.id, blob, a.seqGM); err != nil {
		slog.Error("Failed to save a snapshot", "room", a.id, "error", err)

		return
	}

	a.dirty = false
}

// idle reports whether this room has been empty for longer than the grace
// period. The grace is what makes a refresh instant: a page reload closes the
// socket and opens another one a few hundred milliseconds later, and unloading
// in between would mean a database read and a rehydrate for every F5.
func (a *actor) idle() bool {
	return len(a.conns) == 0 && !a.emptySince.IsZero() &&
		time.Since(a.emptySince) >= a.hub.opts.UnloadGrace
}

// stopAll closes every connection with the same code and reason.
func (a *actor) stopAll(code closeCode, reason string) {
	for c := range a.conns {
		c.stop(code, reason)
		delete(a.conns, c)
	}
	a.emptySince = time.Now()
}

// connected reports whether the user still has a connection in this room, which
// is what tells a closed tab apart from a person leaving.
func (a *actor) connected(user ulid.ULID) bool {
	for c := range a.conns {
		if c.who.ID == user {
			return true
		}
	}

	return false
}

// players is the list the members fragment draws, deep-copied because the
// caller is on another goroutine and the slice underneath is the room's.
func (a *actor) players() []room.Player {
	out := make([]room.Player, len(a.state.Players))
	copy(out, a.state.Players)
	for i := range out {
		if out[i].CharacterID != nil {
			id := *out[i].CharacterID
			out[i].CharacterID = &id
		}
	}

	return out
}

// table is the GM's two configuration dialogs, answered from inside the room
// goroutine because that is the only place the state may be read.
//
// THE PAWN COUNT IS COUNTED HERE AND NOT ASKED FOR LATER. It is the number in
// the layer manager's confirm text -- "Delete Cellar and the 9 pawns on it?" --
// and a caller that received the table and then asked a second question would
// be quoting a count from one instant and deleting from another.
//
// It is every pawn on the layer, not the shown ones. The reader is the GM, and
// what they are about to delete is all of them.
func (a *actor) table() *TableView {
	pawns := make(map[ulid.ULID]int, len(a.state.Table.Layers))
	for _, p := range a.state.Pawns {
		pawns[p.LayerID]++
	}

	return &TableView{Table: room.CloneTable(a.state.Table), Pawns: pawns}
}

// drain answers whatever arrived in the instant between this room deciding to
// retire and removing itself from the hub's map. Every sender selects on done
// as well as on the inbox, so the window is one scheduling gap wide -- but a
// message that got in during it would otherwise wait on a goroutine that has
// already gone.
//
// A join loses the race and is told to go away, which the browser answers by
// reconnecting; the reconnect finds no live room and loads a fresh one. That is
// the same path a deploy takes, and it is why the client's backoff exists.
func (a *actor) drain() {
	for {
		select {
		case m := <-a.inbox:
			switch m := m.(type) {
			case join:
				m.c.stop(closeGoingAway, reasonRestarting)
			case dispatch:
				m.reply <- errGone
			case roster:
				m.reply <- nil
			case tableView:
				m.reply <- nil
			case shutdown:
				close(m.done)
			case closeRoom:
				close(m.done)
			}
		default:
			return
		}
	}
}

// frameType names an event for a log line. The protocol keeps its wire name
// behind an unexported method, deliberately, so this is the Go type -- which is
// what somebody reading the log is going to grep for anyway.
func frameType(ev room.Event) string {
	return fmt.Sprintf("%T", ev)
}
