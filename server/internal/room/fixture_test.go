package room

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE TEST WORLD. Every test in this package builds one of these rather than a
// State literal, for one reason: a State literal is a state somebody asserted
// into existence, and half of what is worth testing here is whether the
// commands can produce it. The world is built by running the same commands the
// hub runs.
//
// IDS ARE A COUNTER, NOT ulid.Make. The whole package is written to be a pure
// function of (state, command), and the one thing that was not -- minting ids
// -- is injected through Env for exactly this. A deterministic id makes a
// golden fixture a diff, an emission list comparable, and a failure message
// something a person can read.

// testID builds a ULID from a small number. The counter goes in the last eight
// bytes so that ids compare in the order they were minted, which is what ULIDs
// do in production and what Normalize's sorting assumes.
func testID(n int) ulid.ULID {
	var id ulid.ULID
	id[0] = 1
	binary.BigEndian.PutUint64(id[8:], uint64(n))

	return id
}

// The fixed identities. They start at a thousand so that nothing minted by an
// Env can collide with one.
var (
	testRoomID    = testID(1000)
	testGMID      = testID(1001)
	testPlayerID  = testID(1002)
	testOtherID   = testID(1003)
	testCharID    = testID(1004)
	testOtherChar = testID(1005)
	testAssetID   = testID(1006)
)

// newEnv is the deterministic id source and a fixed build version.
func newEnv() Env {
	n := 0

	return Env{
		NewID: func() ulid.ULID {
			n++

			return testID(n)
		},
		Version: "test-build",
	}
}

// world is a room with a GM and two players already seated.
type world struct {
	t     *testing.T
	s     *State
	env   Env
	gm    Actor
	pc    Actor
	other Actor
	layer ulid.ULID
}

func newWorld(t *testing.T) *world {
	t.Helper()

	env := newEnv()
	s := NewState(testRoomID, "The Sunless Citadel", env)

	w := &world{
		t:     t,
		s:     s,
		env:   env,
		gm:    Actor{ID: testGMID, Role: RoleGM},
		pc:    Actor{ID: testPlayerID, Role: RolePlayer},
		other: Actor{ID: testOtherID, Role: RolePlayer},
		layer: s.Table.ActiveLayer,
	}

	// Seated through the hub-only command rather than by appending rows, so
	// that the state a test starts from is one the server can actually reach.
	w.apply(&PlayerJoin{Player: Player{ID: testGMID, Name: "Kyle", Role: RoleGM}}, w.gm)
	w.apply(&PlayerJoin{Player: Player{ID: testPlayerID, Name: "Ari", Role: RolePlayer, CharacterID: &testCharID}}, w.gm)
	w.apply(&PlayerJoin{Player: Player{ID: testOtherID, Name: "Rin", Role: RolePlayer, CharacterID: &testOtherChar}}, w.gm)

	return w
}

// run is authorize-then-apply, exactly as the hub will do it, and it is the
// only way a test changes the world. Nothing here reaches into State and
// assigns.
func (w *world) run(c Command, a Actor) ([]Emission, error) {
	w.t.Helper()

	if err := c.Authorize(w.s, a); err != nil {
		return nil, err
	}

	return c.Apply(w.s, a, w.env)
}

// apply is run for the commands a test expects to succeed.
func (w *world) apply(c Command, a Actor) []Emission {
	w.t.Helper()

	ems, err := w.run(c, a)
	if err != nil {
		w.t.Fatalf("%T by %s: unexpected refusal: %v", c, a.Role, err)
	}

	return ems
}

// refuse is run for the commands a test expects to be turned down, and it
// asserts the code as well as the failure -- forbidden and invalid mean
// different things to the client, so a test that accepted either would pass
// while the client did the wrong thing.
func (w *world) refuse(c Command, a Actor, code string) *Error {
	w.t.Helper()

	_, err := w.run(c, a)
	if err == nil {
		w.t.Fatalf("%T by %s: expected %s, got no error", c, a.Role, code)
	}

	e, ok := err.(*Error)
	if !ok {
		w.t.Fatalf("%T by %s: expected a *room.Error, got %T: %v", c, a.Role, err, err)
	}
	if e.Code != code {
		w.t.Fatalf("%T by %s: expected %s, got %s (%s)", c, a.Role, code, e.Code, e.Message)
	}

	return e
}

// spawn puts a pawn on the table through the ordinary command, resolved the way
// the hub would resolve it, and answers with the id it was given.
func (w *world) spawn(p Pawn) ulid.ULID {
	w.t.Helper()

	if p.LayerID.Compare(ulid.ULID{}) == 0 {
		p.LayerID = w.layer
	}
	if p.Kind == "" {
		p.Kind = PawnMonster
	}
	if p.Kind.Creature() && p.Size == "" {
		p.Size = SizeMedium
	}

	before := len(w.s.Pawns)
	w.apply(&PawnSpawn{
		Kind:       p.Kind,
		Layer:      p.LayerID,
		X:          p.X,
		Y:          p.Y,
		Visible:    p.Visible,
		FootprintW: p.FootprintW,
		FootprintH: p.FootprintH,
		Pawn:       &p,
	}, w.gm)

	if len(w.s.Pawns) != before+1 {
		w.t.Fatalf("spawn: pawn count went from %d to %d", before, len(w.s.Pawns))
	}

	// The new pawn is the one with the highest Z, which addPawn sets to one
	// above everything already there.
	var newest Pawn
	for _, q := range w.s.Pawns {
		if q.Z >= newest.Z {
			newest = q
		}
	}

	return newest.ID
}

// addLayer adds a second floor and answers with its id.
func (w *world) addLayer(name string) ulid.ULID {
	w.t.Helper()

	before := len(w.s.Table.Layers)
	w.apply(&TableAddLayer{Name: name}, w.gm)
	if len(w.s.Table.Layers) != before+1 {
		w.t.Fatalf("addLayer: layer count went from %d to %d", before, len(w.s.Table.Layers))
	}

	return w.s.Table.Layers[len(w.s.Table.Layers)-1].ID
}

// THE FAN-OUT, WRITTEN OUT AS A TEST HELPER. This is what phase 3's hub does
// with an emission, and having it here means the audience rules are exercised
// by every test that looks at what somebody received rather than described in a
// comment and implemented once.
func delivered(ems []Emission, actor, viewer Actor) []Event {
	var out []Event

	for _, em := range ems {
		reaches := false
		switch em.To {
		case ToAll:
			reaches = true
		case ToGM:
			reaches = viewer.Role == RoleGM
		case ToPlayers:
			reaches = viewer.Role == RolePlayer
		case ToSender:
			reaches = viewer.ID == actor.ID
		case ToOthers:
			reaches = viewer.ID != actor.ID
		case ToPlayer:
			reaches = viewer.ID == em.Player
		}
		if !reaches {
			continue
		}

		// nil is a real answer from ForRole: this event has nothing to say to
		// this role, and the hub sends nothing rather than an empty payload.
		if ev := ForRole(em.Event, viewer.Role); ev != nil {
			out = append(out, ev)
		}
	}

	return out
}

// summary renders emissions as "type to audience" strings, which is what the
// emission-order assertions compare. Comparing whole events would make every
// test a fixture; comparing types and audiences is what the specification
// actually pins down.
func summary(ems []Emission) []string {
	out := make([]string, 0, len(ems))
	for _, em := range ems {
		out = append(out, em.Event.eventType()+" to "+audienceName(em.To))
	}

	return out
}

func audienceName(a Audience) string {
	switch a {
	case ToAll:
		return "all"
	case ToGM:
		return "gm"
	case ToPlayers:
		return "players"
	case ToSender:
		return "sender"
	case ToOthers:
		return "others"
	case ToPlayer:
		return "player"
	}

	return "?"
}

func eventTypesOf(evs []Event) []string {
	out := make([]string, 0, len(evs))
	for _, ev := range evs {
		out = append(out, ev.eventType())
	}

	return out
}

// equalStrings is the one comparison the emission tests use, spelled out
// because a slice comparison in a table-driven test wants a readable failure.
func equalStrings(t *testing.T, what string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s:\n got %v\nwant %v", what, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s:\n got %v\nwant %v", what, got, want)
		}
	}
}

// mustJSON marshals for the byte comparisons the projection and convergence
// tests are written against.
func mustJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return string(b)
}

func intp(v int) *int            { return &v }
func idp(v ulid.ULID) *ulid.ULID { return &v }
