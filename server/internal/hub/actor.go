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

	// ask is a question put to the room and answered on its goroutine: the
	// player list, the table, one pawn, the tracker, what a spawn needs. The
	// function runs with the state in hand and whatever it returns is sent
	// back; see Hub.view for the typed wrapper every caller goes through.
	//
	// ONE MESSAGE AND NOT ONE PER VIEW, and the reason is drain. Every view
	// used to be a struct of its own with a case in handle and a case in
	// drain, and nothing checked that the two lists agreed -- a view added to
	// one and forgotten in the other was a reply channel nobody wrote, which
	// hung its HTTP caller until the browser gave up, and only when a room
	// happened to be retiring. With one message there is one case in each,
	// and a new view is a closure rather than a type.
	ask struct {
		fn    func(*actor) any
		reply chan any
	}

	// saved is the snapshot writer reporting back: which state it wrote, as
	// the change count the blob was encoded at, and whether the row took it.
	// It arrives on a channel of its own rather than the inbox so that the
	// two synchronous ends of a room can wait for it without handling
	// anything else.
	saved struct {
		changes uint64
		err     error
	}

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

	// THE SNAPSHOT IS WRITTEN OFF THIS GOROUTINE, and these four are how the
	// room knows where that write stands. changes counts every state event
	// since the room loaded; a blob is encoded at one value of it, and dirty
	// clears only when the acknowledged value is still the current one --
	// otherwise something landed while the write was in flight and the next
	// tick writes again. One write is in flight at a time, so a slow database
	// costs the table nothing but a later save; it used to cost every command
	// two seconds while the room waited on the UPDATE.
	changes uint64
	saving  bool
	saved   chan saved

	// saveFailures is how many writes in a row the row has refused, so that
	// a database that has stopped taking snapshots is one line a minute in
	// the log rather than one every five seconds, and so that the count is
	// in it.
	saveFailures int

	// sizeWarned is whether the last blob was over the soft ceiling, so the
	// warning about it is written on the crossing and not on every tick.
	sizeWarned bool

	// sheet writes player pawns' hit points back to their character sheets,
	// and lastHP is what it was last asked to write per character -- so an
	// update that changed a pawn's name, conditions or layer costs no
	// statement at all.
	sheet  *sheetWriter
	lastHP map[ulid.ULID]int

	// drags is the coalescing buffer: at most one pending drag per anchor
	// pawn, flushed on a timer. A fast mouse and a slow one then cost the same
	// bandwidth, which is what keeps the one hot path in the protocol bounded.
	drags     map[ulid.ULID]command
	dragTimer *time.Timer

	// emptySince is when the last connection left, and the zero value means
	// somebody is still here. A room with nobody in it stays loaded for the
	// grace period so that a refresh or a flaky connection is instant.
	emptySince time.Time

	// kicked is who was removed and when, kept for kickGrace after the fact.
	//
	// IT CLOSES THE RACE THE DATABASE CANNOT. A kick clears the person's
	// session rows, and a tab of theirs that was in reconnect backoff at that
	// instant never saw the close reason: it comes back, and if its upgrade
	// read the rows before the clear committed, the membership check passes
	// and join would seat them again. A join for an id in this map is refused
	// with the kick's own reason, whatever the rows said a moment ago.
	kicked map[ulid.ULID]time.Time

	entropy *ulid.MonotonicEntropy
}

// kickGrace is how long a kick is remembered by the room itself. The
// reconnect it exists to refuse arrives within the client's backoff ceiling,
// which is fifteen seconds; a person the GM has removed who is invited back
// with the code inside this window is refused once and tries again.
const kickGrace = 30 * time.Second

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
		kicked:    make(map[ulid.ULID]time.Time),
		saved:     make(chan saved, 1),
		sheet:     newSheetWriter(h.writeHP),
		lastHP:    make(map[ulid.ULID]int),

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

		case m := <-a.saved:
			a.acked(m)

		case <-save.C:
			// The same tick asks both questions, which is why there is one
			// timer rather than two: a room that has been empty for the grace
			// period has just been written back by the line above it, so
			// unloading costs nothing more.
			//
			// A ROOM WITH A WRITE IN FLIGHT OR A WRITE OWED DOES NOT UNLOAD.
			// The first would leave its acknowledgement for nobody; the
			// second is a room whose row has been refusing it, and keeping it
			// in memory until the database takes it is what keeps the table
			// from being lost to an outage that ends an hour later.
			a.save()
			if a.idle() && !a.saving && !a.dirty && a.hub.retire(a) {
				a.sheet.stop()
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

	case ask:
		m.reply <- m.fn(a)

	case shutdown:
		a.stopAll(closeGoingAway, reasonRestarting)
		a.saveNow()
		a.sheet.stop()
		a.hub.forget(a)
		a.drain()
		close(m.done)

		return true

	case closeRoom:
		a.exec(room.Actor{}, &room.RoomClose{}, nil, "")
		a.stopAll(closeNormal, reasonClosed)
		a.saveNow()
		a.sheet.stop()
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
//
// A JOIN CAN BE REFUSED, and a refusal is a close rather than an error frame:
// the connection was never seated, so there is no sequence to stamp a frame
// with and no snapshot for it to precede. Three things refuse. A person the
// room removed inside kickGrace is closed with the kick's reason, which is
// what stops their browser from trying again. Past the per-person cap or the
// room's, the connection is closed with reasonLimit, which the client treats
// the same way -- a fifth tab that reconnected on its backoff for ever would
// be a fifth tab costing the room a snapshot every fifteen seconds.
//
// THE CAPS ARE COUNTED HERE AND NOT IN attach, because conns belongs to this
// goroutine and a count taken anywhere else is a count taken under a race.
func (a *actor) join(c *client) {
	if at, ok := a.kicked[c.who.ID]; ok {
		if time.Since(at) < kickGrace {
			c.stop(closePolicy, reasonKicked)

			return
		}
		delete(a.kicked, c.who.ID)
	}

	if len(a.conns) >= a.hub.opts.ConnsPerRoom || a.count(c.who.ID) >= a.hub.opts.ConnsPerUser {
		slog.Warn("Refusing a connection over the cap", "room", a.id, "user", c.who.ID, "room_conns", len(a.conns))
		c.stop(closePolicy, reasonLimit)

		return
	}

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
			a.changes++
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
	// THE GM'S COPY AND NOT THE PLAYERS', because a shown pawn emits both and
	// this must run once. The GM's is also the unprojected one, so its hit
	// points are the real ones -- the players' copy of a monster carries a band
	// or nothing, and a player pawn's would be right by luck rather than by
	// construction.
	if updated, ok := em.Event.(*room.PawnUpdated); ok && em.To == room.ToGM {
		a.writeThrough(updated.Pawn)

		return
	}

	switch ev := em.Event.(type) {
	case *room.PlayerKicked:
		a.kicked[em.Player] = time.Now()
		a.forget(em.Player)
		a.drop(em.Player, reasonKicked)

	case *room.PlayerLeft:
		// A PERSON WHO PRESSED LEAVE IN ONE TAB MEANT IT IN ALL OF THEM. The
		// row is gone from the state, so a tab of theirs still connected
		// would be a socket whose every command is checked against an id the
		// player list no longer names -- and a reconnect from it would seat
		// them again. After a kick this finds nothing: the kicked event ran
		// first and took the connections with it.
		a.drop(ev.ID, reasonLeft)
	}
}

// drop closes every connection one person has in this room, with one reason.
func (a *actor) drop(user ulid.ULID, reason string) {
	for c := range a.conns {
		if c.who.ID == user {
			c.stop(closePolicy, reason)
			delete(a.conns, c)
		}
	}
	if len(a.conns) == 0 {
		a.emptySince = time.Now()
	}
}

// count is how many connections one person has in this room.
func (a *actor) count(user ulid.ULID) int {
	n := 0
	for c := range a.conns {
		if c.who.ID == user {
			n++
		}
	}

	return n
}

// writeThrough puts a player pawn's hit points back on the character sheet, so
// that a fight fought at the table leaves the sheet saying what happened.
//
// ONE COLUMN AND NO OTHER, which is UpdateCharacterCurrentHP's whole shape.
// Max hit points and armour class were read off the sheet when the pawn was
// spawned and are the TABLE'S copy afterwards: a GM giving one goblin's twin an
// extra point of armour for one fight must not be able to rewrite somebody's
// character, and the same rule applied to a player pawn is what keeps a
// temporary buff out of the sheet.
//
// ONLY WHEN THE NUMBER CHANGED. Every pawn.updated arrives here -- a rename, a
// condition ticking over on initiative.next, a move between floors -- and the
// room remembers what it last asked the writer for per character, so those
// cost nothing. The first update after a load always writes, which is one
// statement that puts the value already there.
//
// OFF THE ROOM'S GOROUTINE AND IN ORDER. A table full of people must not wait
// on an UPDATE, and two edits fifty milliseconds apart must not race each
// other to the row -- "23" then "16" is one damage entry corrected, and a
// sheet that ended up reading 23 would be wrong until the next hit. See
// sheetWriter, which keeps the latest value per character and writes one
// statement at a time.
func (a *actor) writeThrough(p room.Pawn) {
	character, hp, owed := writeThroughHP(p)
	if !owed || a.sheet == nil {
		return
	}
	if last, known := a.lastHP[character]; known && last == hp {
		return
	}

	a.lastHP[character] = hp
	a.sheet.put(character, hp)
}

// writeThroughHP is the decision on its own: whose sheet, what number, and
// whether anything is owed at all.
//
// IT IS SPLIT OUT SO IT CAN BE TESTED WITHOUT A DATABASE. What matters about
// the write-through is which pawns it fires for -- a player's, never a
// monster's, never an object's -- and that is a question about a pawn rather
// than about SQL. The statement above it is one line and the goroutine around
// it is forget's, already established.
func writeThroughHP(p room.Pawn) (ulid.ULID, int, bool) {
	if p.Kind != room.PawnPlayer || p.CharacterID == nil || p.HP == nil {
		return ulid.ULID{}, 0, false
	}

	return *p.CharacterID, *p.HP, true
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

// snapshotSoftLimit is the size past which a room's snapshot is worth a line
// in the log. The point budgets in internal/room keep a room well under it;
// this is the line that says so if something new starts to grow.
const snapshotSoftLimit = 8 << 20

// save starts writing the snapshot when there is something new in it and no
// write is already on its way. The write runs on a goroutine of its own and
// reports back through the saved channel; see acked.
func (a *actor) save() {
	if !a.dirty || a.saving {
		return
	}

	blob, ok := a.encode()
	if !ok {
		return
	}

	a.saving = true
	store, id, seq, changes := a.hub.store, a.id, a.seqGM, a.changes
	report := a.saved

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()

		// The channel has one slot and one write is in flight at a time, so
		// this never blocks -- not even when the room has gone.
		report <- saved{changes: changes, err: store.Save(ctx, id, blob, seq)}
	}()
}

// acked is the writer reporting back. A failure leaves the room dirty, so the
// next tick tries again rather than losing the change; a success clears it
// only when nothing has changed since the blob was encoded.
func (a *actor) acked(m saved) {
	a.saving = false

	if m.err != nil {
		a.saveFailures++
		// The first failure and then one a minute at the default interval,
		// with the count, so a row that has stopped taking snapshots is
		// visible without flooding the log.
		if a.saveFailures == 1 || a.saveFailures%12 == 0 {
			slog.Error("Failed to save a snapshot", "room", a.id, "consecutive", a.saveFailures, "error", m.err)
		}

		return
	}

	a.saveFailures = 0
	if m.changes == a.changes {
		a.dirty = false
	}
}

// saveNow is the synchronous save the two endings of a room make: it waits
// for any write in flight, then writes whatever is still owed on this
// goroutine, because after it returns there is no goroutine to report to.
func (a *actor) saveNow() {
	if a.saving {
		select {
		case m := <-a.saved:
			a.acked(m)
		case <-time.After(storeTimeout + time.Second):
			// The writer's own deadline is storeTimeout, so it has already
			// failed; its report lands in a slot nobody reads again.
			a.saving = false
		}
	}

	if !a.dirty {
		return
	}

	blob, ok := a.encode()
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()

	if err := a.hub.store.Save(ctx, a.id, blob, a.seqGM); err != nil {
		slog.Error("Failed to save a snapshot on the way out", "room", a.id, "error", err)

		return
	}

	a.dirty = false
}

// encode is the blob a save writes, made on this goroutine because it reads
// the state. The state carries its own sequence, which is what a reload
// restores from; the column beside it is for whoever is reading the table by
// hand.
func (a *actor) encode() ([]byte, bool) {
	a.state.Seq = a.seqGM
	blob, err := room.Marshal(a.state)
	if err != nil {
		slog.Error("Failed to encode a snapshot", "room", a.id, "error", err)

		return nil, false
	}

	over := len(blob) > snapshotSoftLimit
	if over && !a.sizeWarned {
		slog.Error("A room's snapshot has grown past the soft ceiling", "room", a.id, "bytes", len(blob))
	}
	a.sizeWarned = over

	return blob, true
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

// pawn is one pawn as the asking role may see it, which is the protocol's own
// question and is therefore asked of the protocol. See room.ProjectedPawn for
// why there is no way to reach the unprojected one from here.
func (a *actor) pawn(id ulid.ULID, role room.Role) *room.Pawn {
	return a.state.ProjectedPawn(id, role)
}

// SpawnView is what resolution needs from a running room: who is at the table
// and what they brought, which floor is active, and the geometry a row of party
// pawns is laid out against.
//
// IT IS READ IN ONE MESSAGE RATHER THAN FOUR. Spawning the party asks who is
// connected, which characters are already standing on the table, where the
// centre of the floor is and how wide a cell is; asking those separately would
// mean four trips into a goroutine that a table full of people is waiting on,
// and four answers from four different instants.
type SpawnView struct {
	ActiveLayer ulid.ULID
	Grid        room.Grid
	Map         *room.MapRef

	// Players is everybody in the room, connected or not. Resolution filters
	// it, because "connected" is its rule rather than this one's.
	Players []room.Player

	// Characters is the set already represented by a player pawn, which is
	// what keeps Spawn pawns from putting a second Ilyana beside the first
	// when it is pressed twice.
	Characters map[ulid.ULID]bool
}

func (a *actor) spawn() *SpawnView {
	view := &SpawnView{
		ActiveLayer: a.state.Table.ActiveLayer,
		Grid:        a.state.Table.Grid,
		Players:     a.players(),
		Characters:  make(map[ulid.ULID]bool),
	}

	if layer := a.state.Layer(view.ActiveLayer); layer != nil && layer.Map != nil {
		m := *layer.Map
		view.Map = &m
	}

	for _, p := range a.state.Pawns {
		if p.Kind == room.PawnPlayer && p.CharacterID != nil {
			view.Characters[*p.CharacterID] = true
		}
	}

	return view
}

// initiative is the tracker, the pawns it names and the table, projected for
// one role and read in one pass on this goroutine.
func (a *actor) initiative(role room.Role) *InitiativeView {
	tracker, pawns := a.state.ProjectedInitiative(role)

	return &InitiativeView{
		Initiative: tracker,
		Pawns:      pawns,
		Table:      a.state.Clone().Table,
	}
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
			case ask:
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
