package hub

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func testID(n uint64) ulid.ULID {
	var id ulid.ULID
	id[0] = 1
	binary.BigEndian.PutUint64(id[8:], n)
	return id
}

var (
	roomID   = testID(1)
	gmID     = testID(2)
	playerID = testID(3)
	otherID  = testID(4)
)

type memStore struct {
	mu        sync.Mutex
	loaded    Loaded
	loadErr   error
	saves     [][]byte
	seqs      []uint64
	saveErr   error
	saveDelay time.Duration
	inFlight  int
	cleared   [][2]ulid.ULID
	preserved [][]byte
	scenes    [][]byte
}

func (m *memStore) Load(ctx context.Context, id ulid.ULID) (Loaded, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loaded, m.loadErr
}
func (m *memStore) Save(ctx context.Context, id ulid.ULID, blob []byte, seq uint64) error {
	m.mu.Lock()
	delay := m.saveDelay
	m.inFlight++
	m.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight--
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saves = append(m.saves, blob)
	m.seqs = append(m.seqs, seq)
	return nil
}
func (m *memStore) saving() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inFlight
}
func (m *memStore) failSaves(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveErr = err
}
func (m *memStore) ClearMembership(ctx context.Context, id, user ulid.ULID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleared = append(m.cleared, [2]ulid.ULID{id, user})
	return nil
}
func (m *memStore) Preserve(ctx context.Context, id ulid.ULID, snapshot []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preserved = append(m.preserved, snapshot)
	return nil
}
func (m *memStore) kept() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte(nil), m.preserved...)
}
func (m *memStore) saved() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.saves)
}
func (m *memStore) AutosaveScene(ctx context.Context, id ulid.ULID, body []byte, preview *ulid.ULID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenes = append(m.scenes, body)
	return nil
}
func (m *memStore) autosaved() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte(nil), m.scenes...)
}
func (m *memStore) clears() [][2]ulid.ULID {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][2]ulid.ULID(nil), m.cleared...)
}

type tabletop struct {
	t *testing.T
	*Hub
	store *memStore
}

func newTabletop(t *testing.T, opts Options) *tabletop {
	t.Helper()
	store := &memStore{loaded: Loaded{Name: "The Sunless Citadel"}}
	if opts.Store == nil {
		opts.Store = store
	}
	if opts.SnapshotInterval == 0 {
		opts.SnapshotInterval = time.Hour
	}
	if opts.UnloadGrace == 0 {
		opts.UnloadGrace = time.Hour
	}
	if opts.Version == "" {
		opts.Version = "test-build"
	}
	h := New(nil, opts)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		h.Shutdown(ctx)
	})
	return &tabletop{t: t, Hub: h, store: store}
}
func (tb *tabletop) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	tb.t.Cleanup(cancel)
	return ctx
}
func (tb *tabletop) actor() *actor {
	tb.t.Helper()
	a, err := tb.room(tb.ctx(), roomID)
	if err != nil {
		tb.t.Fatalf("the room would not load: %v", err)
	}
	return a
}
func (tb *tabletop) join(id ulid.ULID, name string, role room.Role) *client {
	tb.t.Helper()
	a := tb.actor()
	c := newClient(room.Player{ID: id, Name: name, Role: role}, tb.opts.SendBuffer)
	if err := a.post(tb.ctx(), join{c: c}); err != nil {
		tb.t.Fatalf("join was refused: %v", err)
	}
	tb.settle()
	return c
}
func (tb *tabletop) seat(user, character ulid.ULID, name string) {
	tb.t.Helper()
	err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.PlayerJoin{
		Player: room.Player{ID: user, Name: name, Role: room.RolePlayer, CharacterID: &character},
	})
	if err != nil {
		tb.t.Fatalf("seating %s: %v", name, err)
	}
}
func (tb *tabletop) send(c *client, cid string, cmd room.Command) {
	tb.t.Helper()
	if err := tb.actor().post(tb.ctx(), command{c: c, cmd: cmd, cid: cid}); err != nil {
		tb.t.Fatalf("the command was refused: %v", err)
	}
	tb.settle()
}
func (tb *tabletop) leave(c *client) {
	tb.t.Helper()
	if err := tb.actor().post(tb.ctx(), leave{c: c}); err != nil {
		tb.t.Fatalf("leave was refused: %v", err)
	}
	tb.settle()
}
func (tb *tabletop) flush() {
	tb.t.Helper()
	reply := make(chan any, 1)
	if err := tb.actor().post(tb.ctx(), ask{fn: func(a *actor) any { a.flushPending(); return nil }, reply: reply}); err != nil {
		tb.t.Fatalf("the coalescing window would not close: %v", err)
	}
	<-reply
}
func (tb *tabletop) settle() {
	tb.t.Helper()
	if _, ok := tb.Players(tb.ctx(), roomID); !ok {
		tb.t.Fatal("the room was not live when the test tried to synchronise with it")
	}
}

type frame struct {
	Type string
	Seq  uint64
	CID  string
	Body map[string]any
}

func frames(t *testing.T, c *client) []frame {
	t.Helper()
	var out []frame
	for {
		select {
		case raw := <-c.out:
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("a frame was not JSON: %v\n%s", err, raw)
			}
			f := frame{Body: body}
			f.Type, _ = body["type"].(string)
			f.CID, _ = body["cid"].(string)
			if seq, ok := body["seq"].(float64); ok {
				f.Seq = uint64(seq)
			}
			out = append(out, f)
		default:
			return out
		}
	}
}
func types(fs []frame) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Type)
	}
	return out
}
func events(t *testing.T, c *client) []frame {
	t.Helper()
	var out []frame
	for _, f := range frames(t, c) {
		if f.Type != "changes" {
			out = append(out, f)
			continue
		}
		carried, ok := f.Body["events"].([]any)
		if !ok {
			t.Fatalf("a changes frame carries no events: %+v", f.Body)
		}
		for _, one := range carried {
			body, ok := one.(map[string]any)
			if !ok {
				t.Fatalf("an event inside a frame is not an object: %v", one)
			}
			inner := frame{Seq: f.Seq, Body: body}
			inner.Type, _ = body["type"].(string)
			out = append(out, inner)
		}
	}
	return out
}
func only(t *testing.T, c *client, want ...string) []frame {
	t.Helper()
	got := events(t, c)
	if !equal(types(got), want) {
		t.Fatalf("events = %v, want %v", types(got), want)
	}
	return got
}
func onePawn(t *testing.T, f frame) map[string]any {
	t.Helper()
	pawns, ok := f.Body["pawns"].([]any)
	if !ok || len(pawns) != 1 {
		t.Fatalf("the upsert carries %v, want one pawn", f.Body["pawns"])
	}
	pawn, ok := pawns[0].(map[string]any)
	if !ok {
		t.Fatalf("the upserted pawn is not an object: %v", pawns[0])
	}
	return pawn
}
func oneID(t *testing.T, f frame) ulid.ULID {
	t.Helper()
	ids, ok := f.Body["ids"].([]any)
	if !ok || len(ids) != 1 {
		t.Fatalf("the change names %v, want one id", f.Body["ids"])
	}
	raw, _ := ids[0].(string)
	id, err := ulid.Parse(raw)
	if err != nil {
		t.Fatalf("the change names %q, which is not a ULID", raw)
	}
	return id
}
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
