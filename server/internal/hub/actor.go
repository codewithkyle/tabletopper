package hub

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"time"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

type (
	join    struct{ c *client }
	leave   struct{ c *client }
	command struct {
		c   *client
		cmd room.Command
		cid string
		err error
	}
	dispatch struct {
		who   room.Actor
		cmd   room.Command
		reply chan error
	}
	ask struct {
		fn    func(*actor) any
		reply chan any
	}
	saved struct {
		changes uint64
		err     error
	}
	shutdown  struct{ done chan struct{} }
	closeRoom struct{ done chan struct{} }
)
type actor struct {
	hub              *Hub
	id               ulid.ULID
	inbox            chan any
	done             chan struct{}
	state            *room.State
	conns            map[*client]struct{}
	seqGM, seqPlayer uint64
	dirty            bool
	changes          uint64
	saving           bool
	saved            chan saved
	saveFailures     int
	sizeWarned       bool
	sheet            *sheetWriter
	lastHP           map[ulid.ULID]int
	secret           map[ulid.ULID][]room.Roll
	pending          map[string]command
	coalesceTimer    *time.Timer
	emptySince       time.Time
	kicked           map[ulid.ULID]time.Time
	entropy          *ulid.MonotonicEntropy
	startedAt        time.Time
	savedAt          time.Time
	lastBytes        int
}

const kickGrace = 30 * time.Second

func newActor(h *Hub, id ulid.ULID, state *room.State, seq uint64) *actor {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	return &actor{
		hub:           h,
		id:            id,
		inbox:         make(chan any, inboxSize),
		done:          make(chan struct{}),
		state:         state,
		conns:         make(map[*client]struct{}),
		seqGM:         seq,
		seqPlayer:     seq,
		pending:       make(map[string]command),
		coalesceTimer: timer,
		kicked:        make(map[ulid.ULID]time.Time),
		saved:         make(chan saved, 1),
		sheet:         newSheetWriter(h.writeHP),
		lastHP:        make(map[ulid.ULID]int),
		secret:        make(map[ulid.ULID][]room.Roll),
		emptySince:    time.Now(),
		entropy:       ulid.Monotonic(rand.Reader, 0),
		startedAt:     time.Now(),
	}
}
func (a *actor) env() room.Env {
	return room.Env{
		NewID:   func() ulid.ULID { return ulid.MustNew(ulid.Now(), a.entropy) },
		Version: a.hub.version,
	}
}
func (a *actor) run() {
	save := time.NewTicker(a.hub.opts.SnapshotInterval)
	defer save.Stop()
	defer a.coalesceTimer.Stop()
	for {
		select {
		case m := <-a.inbox:
			if a.handle(m) {
				return
			}
		case <-a.coalesceTimer.C:
			a.flushPending()
		case m := <-a.saved:
			a.acked(m)
		case <-save.C:
			a.save()
			if a.idle() && !a.saving && !a.dirty && a.hub.retire(a) {
				a.sheet.stop()
				a.drain()
				return
			}
		}
	}
}
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
func (a *actor) fromClient(m command) {
	if m.err != nil {
		a.refuse(m.c, m.cid, m.err)
		return
	}
	if c, ok := m.cmd.(room.Coalescer); ok {
		if len(a.pending) == 0 {
			a.coalesceTimer.Reset(a.hub.opts.CoalesceInterval)
		}
		a.pending[c.CoalesceKey()] = m
		return
	}
	if err := a.exec(m.c.who, m.cmd, m.c, m.cid); err != nil {
		a.refuse(m.c, m.cid, err)
	}
}
func (a *actor) flushPending() {
	keys := make([]string, 0, len(a.pending))
	for key := range a.pending {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		m := a.pending[key]
		delete(a.pending, key)
		if err := a.exec(m.c.who, m.cmd, m.c, m.cid); err != nil {
			a.refuse(m.c, m.cid, err)
		}
	}
}
func (a *actor) exec(who room.Actor, cmd room.Command, sender *client, cid string) error {
	if err := cmd.Authorize(a.state, who); err != nil {
		return err
	}
	if _, ok := cmd.(room.Preview); ok {
		return a.preview(who, cmd, sender)
	}
	before := a.state.Clone()
	sigs, err := cmd.Apply(a.state, who, a.env())
	if err != nil {
		a.state = &before
		return err
	}
	derived := a.broadcast(&before, who)
	a.emit(sigs, who, sender, nil)
	a.signals(sigs, who)
	a.changed(&before, derived)
	return nil
}
func (a *actor) preview(who room.Actor, cmd room.Command, sender *client) error {
	sigs, err := cmd.Apply(a.state, who, a.env())
	if err != nil {
		return err
	}
	a.emit(sigs, who, sender, nil)
	return nil
}
func (a *actor) broadcast(before *room.State, who room.Actor) []room.Change {
	if reflect.DeepEqual(*before, *a.state) {
		return nil
	}
	a.dirty = true
	a.changes++
	by := actorID(who)
	var gm []room.Change
	for _, role := range []room.Role{room.RoleGM, room.RolePlayer} {
		changes := room.Derive(before, a.state, role)
		if role == room.RoleGM {
			gm = changes
		}
		if len(changes) == 0 {
			continue
		}
		frame, err := room.EncodeEvent(room.NewChanges(changes), a.advance(role, true), by)
		if err != nil {
			slog.Error("Failed to encode a frame", "room", a.id, "role", role, "error", err)
			continue
		}
		for _, c := range a.byRole(role) {
			a.send(c, frame)
		}
	}
	return gm
}
func (a *actor) emit(sigs []room.Signal, who room.Actor, sender, only *client) {
	by := actorID(who)
	for _, sig := range sigs {
		for _, role := range a.audience(sig, who) {
			seq := a.advance(role, false)
			ev := room.ProjectSignal(a.state, sig, role)
			if ev == nil {
				continue
			}
			if snap, ok := ev.(*room.Snapshot); ok {
				snap.State.Seq = seq
			}
			frame, err := room.EncodeEvent(ev, seq, by)
			if err != nil {
				slog.Error("Failed to encode an event", "room", a.id, "event", frameType(ev), "error", err)
				continue
			}
			for _, c := range a.recipients(sig, role, who, sender, only) {
				a.send(c, frame)
			}
		}
	}
}
func actorID(who room.Actor) *ulid.ULID {
	if who.ID.IsZero() {
		return nil
	}
	id := who.ID
	return &id
}
func (a *actor) audience(sig room.Signal, who room.Actor) []room.Role {
	switch sig.To {
	case room.ToAll, room.ToOthers:
		return []room.Role{room.RoleGM, room.RolePlayer}
	case room.ToSender:
		return []room.Role{who.Role}
	case room.ToPlayer:
		var roles []room.Role
		for c := range a.conns {
			if c.who.ID == sig.Player && !slices.Contains(roles, c.who.Role) {
				roles = append(roles, c.who.Role)
			}
		}
		return roles
	}
	return nil
}
func (a *actor) byRole(role room.Role) []*client {
	var out []*client
	for c := range a.conns {
		if c.who.Role == role {
			out = append(out, c)
		}
	}
	return out
}
func (a *actor) recipients(sig room.Signal, role room.Role, who room.Actor, sender, only *client) []*client {
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
		switch sig.To {
		case room.ToSender:
			if c.who.ID != who.ID {
				continue
			}
		case room.ToOthers:
			if c == sender {
				continue
			}
		case room.ToPlayer:
			if c.who.ID != sig.Player {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
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
func (a *actor) send(c *client, frame []byte) {
	if c.enqueue(frame) {
		return
	}
	slog.Warn("Dropping a connection that cannot keep up", "room", a.id, "user", c.who.ID)
	c.stop(closePolicy, reasonSlow)
	delete(a.conns, c)
}
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
func (a *actor) snapshot(c *client) {
	sigs, err := (&room.SyncRequest{}).Apply(a.state, c.who, a.env())
	if err != nil {
		slog.Error("Failed to build a snapshot", "room", a.id, "error", err)
		return
	}
	a.emit(sigs, c.who, c, c)
}
func (a *actor) signals(sigs []room.Signal, who room.Actor) {
	for _, sig := range sigs {
		switch ev := sig.Event.(type) {
		case *room.PlayerKicked:
			a.kicked[sig.Player] = time.Now()
			a.forget(sig.Player)
			a.drop(sig.Player, reasonKicked)
		case *room.Rolled:
			a.keepSecret(who.ID, ev.Roll)
		}
	}
}
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
func (a *actor) count(user ulid.ULID) int {
	n := 0
	for c := range a.conns {
		if c.who.ID == user {
			n++
		}
	}
	return n
}
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

const snapshotSoftLimit = 8 << 20

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
		report <- saved{changes: changes, err: store.Save(ctx, id, blob, seq)}
	}()
}
func (a *actor) acked(m saved) {
	a.saving = false
	if m.err != nil {
		a.saveFailures++
		if a.saveFailures == 1 || a.saveFailures%12 == 0 {
			slog.Error("Failed to save a snapshot", "room", a.id, "consecutive", a.saveFailures, "error", m.err)
		}
		return
	}
	a.saveFailures = 0
	a.savedAt = time.Now()
	if m.changes == a.changes {
		a.dirty = false
	}
}
func (a *actor) saveNow() {
	if a.saving {
		select {
		case m := <-a.saved:
			a.acked(m)
		case <-time.After(storeTimeout + time.Second):
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
	a.savedAt = time.Now()
	a.dirty = false
}
func (a *actor) encode() ([]byte, bool) {
	a.state.Seq = a.seqGM
	blob, err := room.Marshal(a.state)
	if err != nil {
		slog.Error("Failed to encode a snapshot", "room", a.id, "error", err)
		return nil, false
	}
	a.lastBytes = len(blob)
	over := len(blob) > snapshotSoftLimit
	if over && !a.sizeWarned {
		slog.Error("A room's snapshot has grown past the soft ceiling", "room", a.id, "bytes", len(blob))
	}
	a.sizeWarned = over
	return blob, true
}
func (a *actor) idle() bool {
	return len(a.conns) == 0 && !a.emptySince.IsZero() &&
		time.Since(a.emptySince) >= a.hub.opts.UnloadGrace
}
func (a *actor) stopAll(code closeCode, reason string) {
	for c := range a.conns {
		c.stop(code, reason)
		delete(a.conns, c)
	}
	a.emptySince = time.Now()
}
func (a *actor) connected(user ulid.ULID) bool {
	for c := range a.conns {
		if c.who.ID == user {
			return true
		}
	}
	return false
}
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
func (a *actor) table() *TableView {
	pawns := make(map[ulid.ULID]int, len(a.state.Table.Layers))
	for _, p := range a.state.Pawns {
		pawns[p.LayerID]++
	}
	return &TableView{Table: room.CloneTable(a.state.Table), Pawns: pawns}
}
func (a *actor) pawn(id ulid.ULID, role room.Role) *room.Pawn {
	return a.state.ProjectedPawn(id, role)
}
func (a *actor) debug() *DebugView {
	perUser := make(map[ulid.ULID]int, len(a.conns))
	for c := range a.conns {
		perUser[c.who.ID]++
	}
	conns := make([]DebugConn, 0, len(perUser))
	for user, count := range perUser {
		conns = append(conns, DebugConn{User: user.String(), Count: count})
	}
	slices.SortFunc(conns, func(x, y DebugConn) int { return strings.Compare(x.User, y.User) })
	keys := make([]string, 0, len(a.pending))
	for key := range a.pending {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return &DebugView{
		Conns:         len(a.conns),
		PerUser:       conns,
		SeqGM:         a.seqGM,
		SeqPlayer:     a.seqPlayer,
		Changes:       a.changes,
		Dirty:         a.dirty,
		Saving:        a.saving,
		SaveFailures:  a.saveFailures,
		SnapshotBytes: a.lastBytes,
		SoftLimit:     snapshotSoftLimit,
		SavedAt:       a.savedAt,
		StartedAt:     a.startedAt,
		EmptySince:    a.emptySince,
		Coalescing:    keys,
		Kicked:        len(a.kicked),
		Inbox:         len(a.inbox),
		InboxCap:      cap(a.inbox),
		Players:       len(a.state.Players),
		Pawns:         len(a.state.Pawns),
		Layers:        len(a.state.Table.Layers),
		Fog:           len(a.state.Fog),
		Strokes:       len(a.state.Strokes),
	}
}

func (a *actor) initiative(role room.Role) *InitiativeView {
	tracker, pawns := a.state.ProjectedInitiative(role)
	return &InitiativeView{
		Initiative: tracker,
		Pawns:      pawns,
		Table:      a.state.Clone().Table,
	}
}
func (a *actor) rolls(viewer ulid.ULID) *RollsView {
	return &RollsView{Rolls: room.MergeRolls(a.state.Rolls, a.secret[viewer])}
}
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
func frameType(ev room.Event) string {
	return fmt.Sprintf("%T", ev)
}
