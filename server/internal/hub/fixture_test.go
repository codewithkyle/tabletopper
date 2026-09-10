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

// THE HARNESS. Every test below drives a real room through real commands --
// the actor goroutine, the sequence counters, the projection, the emissions --
// with two things replaced: the database is a map, and a connection is a
// channel. Neither replacement is a stub of behaviour, which is why these are
// unit tests of the hub rather than an integration suite. The one test that
// does need a socket is in socket_test.go and there is exactly one of it.

// testID is a readable ULID: the low eight bytes are the number, so ids sort in
// the order a test wrote them and a failure message says which one it is.
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

// memStore is the rooms row, in memory. It records what it was asked to write
// so the persistence tests can assert on the calls rather than on a database.
type memStore struct {
	mu      sync.Mutex
	loaded  Loaded
	loadErr error

	saves   [][]byte
	seqs    []uint64
	saveErr error

	// saveDelay holds every Save for that long before it answers, and
	// inFlight counts the ones held, for the test that asks whether the room
	// waits on its database.
	saveDelay time.Duration
	inFlight  int

	cleared [][2]ulid.ULID

	// preserved is every snapshot the hub asked to keep because it could not
	// read it.
	preserved [][]byte
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

func (m *memStore) clears() [][2]ulid.ULID {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([][2]ulid.ULID(nil), m.cleared...)
}

// tabletop is one hub with one room in it and the helpers to push things
// through it.
type tabletop struct {
	t *testing.T
	*Hub
	store *memStore
}

// newTabletop builds the hub. The two intervals default to longer than any test
// runs, so a test that says nothing about saving or unloading never does either
// and a test that is about one of them sets that one.
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

// actor loads the room, which every helper below needs and no test asserts on.
func (tb *tabletop) actor() *actor {
	tb.t.Helper()

	a, err := tb.room(tb.ctx(), roomID)
	if err != nil {
		tb.t.Fatalf("the room would not load: %v", err)
	}

	return a
}

// join seats one connection and returns it. The frames its snapshot and the
// join event produced are still in its buffer afterwards, because that is what
// half the tests here are about.
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

// send posts one command as if it had arrived on that connection.
func (tb *tabletop) send(c *client, cid string, cmd room.Command) {
	tb.t.Helper()

	if err := tb.actor().post(tb.ctx(), command{c: c, cmd: cmd, cid: cid}); err != nil {
		tb.t.Fatalf("the command was refused: %v", err)
	}
	tb.settle()
}

// leave drops one connection.
func (tb *tabletop) leave(c *client) {
	tb.t.Helper()

	if err := tb.actor().post(tb.ctx(), leave{c: c}); err != nil {
		tb.t.Fatalf("leave was refused: %v", err)
	}
	tb.settle()
}

// settle waits for the room to have answered everything sent before this call.
//
// IT IS A ROUND TRIP AND NOT A SLEEP. The inbox is a channel, so a message the
// actor answers is a message every earlier one has already been answered --
// asking for the player list and waiting for the reply is therefore a fence,
// and a test that used a sleep instead would be a test that fails on a loaded
// machine.
func (tb *tabletop) settle() {
	tb.t.Helper()

	if _, ok := tb.Players(tb.ctx(), roomID); !ok {
		tb.t.Fatal("the room was not live when the test tried to synchronise with it")
	}
}

// frame is one decoded event off a connection, with the three envelope fields
// every assertion here reads and the whole object for the ones that read more.
type frame struct {
	Type string
	Seq  uint64
	CID  string
	Body map[string]any
}

// frames drains everything queued for a connection.
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

// types is the frame names in order, which is what most assertions compare.
func types(fs []frame) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Type)
	}

	return out
}

// only asserts that a connection received exactly these frame types, in order.
func only(t *testing.T, c *client, want ...string) []frame {
	t.Helper()

	got := frames(t, c)
	if !equal(types(got), want) {
		t.Fatalf("frames = %v, want %v", types(got), want)
	}

	return got
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

// eventually retries until the condition holds or the deadline passes. It is
// for the three things in this package that are genuinely timer-driven -- the
// save tick, the unload grace and the drag flush -- and for nothing else.
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
