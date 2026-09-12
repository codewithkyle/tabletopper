package room

import (
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func testID(n int) ulid.ULID {
	var id ulid.ULID
	id[0] = 1
	binary.BigEndian.PutUint64(id[8:], uint64(n))
	return id
}

var (
	testRoomID    = testID(1000)
	testGMID      = testID(1001)
	testPlayerID  = testID(1002)
	testOtherID   = testID(1003)
	testCharID    = testID(1004)
	testOtherChar = testID(1005)
	testAssetID   = testID(1006)
	testTrackID   = testID(1007)
)

const testClockStart = 1_767_225_600_000
const testClockStep = 1_000

func newEnv() Env {
	n, faces, ticks := 0, 0, 0
	return Env{
		NewID: func() ulid.ULID {
			n++
			return testID(n)
		},
		Dice: func(sides int) int {
			faces++
			return (faces-1)%sides + 1
		},
		Now: func() time.Time {
			ticks++
			return time.UnixMilli(testClockStart + int64(ticks*testClockStep)).UTC()
		},
		Version: "test-build",
	}
}

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
	w.apply(&PlayerJoin{Player: Player{ID: testGMID, Name: "Kyle", Role: RoleGM}}, w.gm)
	w.apply(&PlayerJoin{Player: Player{ID: testPlayerID, Name: "Ari", Role: RolePlayer, CharacterID: &testCharID, CharacterName: "Ilyana"}}, w.gm)
	w.apply(&PlayerJoin{Player: Player{ID: testOtherID, Name: "Rin", Role: RolePlayer, CharacterID: &testOtherChar, CharacterName: "Brannor"}}, w.gm)
	return w
}
func (w *world) run(c Command, a Actor) ([]Signal, error) {
	w.t.Helper()
	if err := c.Authorize(w.s, a); err != nil {
		return nil, err
	}
	return c.Apply(w.s, a, w.env)
}
func (w *world) apply(c Command, a Actor) []Signal {
	w.t.Helper()
	sigs, err := w.run(c, a)
	if err != nil {
		w.t.Fatalf("%T by %s: unexpected refusal: %v", c, a.Role, err)
	}
	return sigs
}
func (w *world) change(c Command, a Actor) change {
	w.t.Helper()
	before := w.s.Clone()
	return change{w: w, actor: a, before: before, signals: w.apply(c, a)}
}

type change struct {
	w       *world
	actor   Actor
	before  State
	signals []Signal
}

func (ch change) changes(role Role) []Change {
	return Derive(&ch.before, ch.w.s, role)
}
func (ch change) sent(viewer Actor) []Event {
	var out []Event
	if chs := ch.changes(viewer.Role); len(chs) > 0 {
		out = append(out, NewChanges(chs))
	}
	for _, sig := range ch.signals {
		if !reaches(sig, ch.actor, viewer) {
			continue
		}
		if ev := ProjectSignal(ch.w.s, sig, viewer.Role); ev != nil {
			out = append(out, ev)
		}
	}
	return out
}
func reaches(sig Signal, actor, viewer Actor) bool {
	switch sig.To {
	case ToAll:
		return true
	case ToSender:
		return viewer.ID == actor.ID
	case ToOthers:
		return viewer.ID != actor.ID
	case ToPlayer:
		return viewer.ID == sig.Player
	}
	return false
}
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
		Kind:    p.Kind,
		Layer:   p.LayerID,
		X:       p.X,
		Y:       p.Y,
		Visible: p.Visible,
		Pawn:    &p,
	}, w.gm)
	if len(w.s.Pawns) != before+1 {
		w.t.Fatalf("spawn: pawn count went from %d to %d", before, len(w.s.Pawns))
	}
	return newestPawn(w.s)
}
func newestPawn(s *State) ulid.ULID {
	var newest Pawn
	for _, q := range s.Pawns {
		if q.Z >= newest.Z {
			newest = q
		}
	}
	return newest.ID
}
func (w *world) addLayer(name string) ulid.ULID {
	w.t.Helper()
	before := len(w.s.Table.Layers)
	w.apply(&TableAddLayer{Name: name}, w.gm)
	if len(w.s.Table.Layers) != before+1 {
		w.t.Fatalf("addLayer: layer count went from %d to %d", before, len(w.s.Table.Layers))
	}
	return w.s.Table.Layers[len(w.s.Table.Layers)-1].ID
}
func summary(sigs []Signal) []string {
	out := make([]string, 0, len(sigs))
	for _, sig := range sigs {
		out = append(out, sig.Event.eventType()+" to "+audienceName(sig.To))
	}
	return out
}
func audienceName(a Audience) string {
	switch a {
	case ToAll:
		return "all"
	case ToSender:
		return "sender"
	case ToOthers:
		return "others"
	case ToPlayer:
		return "player"
	}
	return "?"
}
func changeTypesOf(chs []Change) []string {
	out := make([]string, 0, len(chs))
	for _, ch := range chs {
		out = append(out, ch.changeType())
	}
	return out
}
func eventTypesOf(evs []Event) []string {
	out := make([]string, 0, len(evs))
	for _, ev := range evs {
		out = append(out, ev.eventType())
	}
	return out
}
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
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
func intp(v int) *int            { return &v }
func strp(v string) *string      { return &v }
func idp(v ulid.ULID) *ulid.ULID { return &v }
