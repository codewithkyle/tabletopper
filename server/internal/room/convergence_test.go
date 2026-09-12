package room

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/oklog/ulid/v2"
)

var update = flag.Bool("update", false, "rewrite the reducer fixtures in testdata")

func TestDerivedEventsConvergeOnTheServersState(t *testing.T) {
	w := newWorld(t)
	r := &recorder{t: t, w: w}
	r.visit = func(st stepRecord) {
		if got, want := mustJSON(t, w.s), mustJSON(t, w.s.Clone()); got != want {
			t.Fatalf("%s: the state it left behind is not normalized\n  got %s\n want %s", st.name, got, want)
		}
		for _, v := range r.viewers() {
			got := v.before.Clone()
			for _, ev := range st.change.sent(v.actor) {
				if err := replay(&got, ev); err != nil {
					t.Fatalf("%s: replaying %s for the %s: %v", st.name, ev.eventType(), v.actor.Role, err)
				}
			}
			want := w.s.Project(v.actor.Role)
			if mustJSON(t, got) != mustJSON(t, want) {
				t.Fatalf("%s: the %s's reduced state is not the server's\n reduced %s\n  server %s",
					st.name, v.actor.Role, mustJSON(t, got), mustJSON(t, want))
			}
			for _, ch := range st.change.changes(v.actor.Role) {
				alone := v.before.Clone()
				if err := Reduce(&alone, ch); err != nil {
					t.Fatalf("%s: reducing %s for the %s: %v", st.name, ch.changeType(), v.actor.Role, err)
				}
				if mustJSON(t, alone) == mustJSON(t, v.before) {
					t.Fatalf("%s: %s changes nothing for the %s; the derivation is sending a no-op",
						st.name, ch.changeType(), v.actor.Role)
				}
			}
		}
	}
	scenario(r)
	if r.steps < 50 {
		t.Fatalf("the scenario ran %d commands; it is meant to cover the whole catalog", r.steps)
	}
	if missing := r.uncovered(); len(missing) > 0 {
		t.Fatalf("the scenario never runs: %v", missing)
	}
}
func TestReducerFixturesAreCurrent(t *testing.T) {
	for _, role := range []Role{RoleGM, RolePlayer} {
		t.Run(string(role), func(t *testing.T) {
			w := newWorld(t)
			fixture := reducerFixture{Role: string(role), Initial: w.s.Project(role)}
			seq := uint64(0)
			r := &recorder{t: t, w: w}
			viewer := Actor{ID: testGMID, Role: RoleGM}
			if role == RolePlayer {
				viewer = Actor{ID: testPlayerID, Role: RolePlayer}
			}
			r.visit = func(st stepRecord) {
				frames := []json.RawMessage{}
				for _, ev := range st.change.sent(viewer) {
					if _, transient := ev.(Transient); !transient {
						seq++
					}
					var by *ulid.ULID
					if !st.hub {
						id := st.actor.ID
						by = &id
					}
					b, err := EncodeEvent(ev, seq, by)
					if err != nil {
						t.Fatalf("%s: encode: %v", st.name, err)
					}
					frames = append(frames, json.RawMessage(b))
				}
				fixture.Steps = append(fixture.Steps, reducerStep{
					Name:   st.name,
					Frames: frames,
					State:  w.s.Project(role),
				})
			}
			scenario(r)
			got, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "reducer", string(role)+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v -- run `go test ./internal/room -update` to write it", err)
			}
			if string(got) != string(want) {
				t.Fatalf("%s is stale; run `go test ./internal/room -update` and read the diff", path)
			}
		})
	}
}
func TestTheFixturesReplayFromTheirInitialState(t *testing.T) {
	for _, role := range []Role{RoleGM, RolePlayer} {
		t.Run(string(role), func(t *testing.T) {
			var fixture reducerFixture
			b, err := os.ReadFile(filepath.Join("testdata", "reducer", string(role)+".json"))
			if err != nil {
				t.Fatalf("%v -- run `go test ./internal/room -update` to write it", err)
			}
			if err := json.Unmarshal(b, &fixture); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			state := fixture.Initial.Clone()
			for _, step := range fixture.Steps {
				for _, raw := range step.Frames {
					ev, err := decodeEventForTest(raw)
					if err != nil {
						t.Fatalf("%s: %v", step.Name, err)
					}
					if err := replay(&state, ev); err != nil {
						t.Fatalf("%s: %v", step.Name, err)
					}
				}
				if mustJSON(t, state) != mustJSON(t, step.State) {
					t.Fatalf("%s: the replayed state has drifted\n replay %s\nfixture %s",
						step.Name, mustJSON(t, state), mustJSON(t, step.State))
				}
			}
		})
	}
}

type reducerFixture struct {
	Role    string        `json:"role"`
	Initial State         `json:"initial"`
	Steps   []reducerStep `json:"steps"`
}
type reducerStep struct {
	Name   string            `json:"name"`
	Frames []json.RawMessage `json:"frames"`
	State  State             `json:"state"`
}

func replay(s *State, ev Event) error {
	switch e := ev.(type) {
	case *Changes:
		for _, ch := range e.Events {
			if err := Reduce(s, ch); err != nil {
				return err
			}
		}
	case *Snapshot:
		*s = e.State.Clone()
	}
	return nil
}

func decodeEventForTest(raw json.RawMessage) (Event, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	ev, ok := EventPrototypes()[envelope.Type]
	if !ok {
		return nil, &Error{Code: CodeInvalid, Message: "unknown event " + envelope.Type}
	}
	if err := json.Unmarshal(raw, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

type stepRecord struct {
	name   string
	actor  Actor
	hub    bool
	change change
	before map[Role]State
}
type recorder struct {
	t       *testing.T
	w       *world
	visit   func(stepRecord)
	steps   int
	ran     map[string]bool
	current map[Role]State
}
type viewer struct {
	actor  Actor
	before State
}

func (r *recorder) viewers() []viewer {
	return []viewer{
		{actor: r.w.gm, before: r.current[RoleGM]},
		{actor: r.w.pc, before: r.current[RolePlayer]},
	}
}
func (r *recorder) do(name string, cmd Command, actor Actor) {
	r.t.Helper()
	r.record(name, cmd, actor, false)
}
func (r *recorder) hub(name string, cmd Command) {
	r.t.Helper()
	r.record(name, cmd, r.w.gm, true)
}
func (r *recorder) record(name string, cmd Command, actor Actor, hub bool) {
	r.t.Helper()
	if r.ran == nil {
		r.ran = map[string]bool{}
	}
	r.ran[commandWire(cmd)] = true
	r.steps++
	r.current = map[Role]State{
		RoleGM:     r.w.s.Project(RoleGM),
		RolePlayer: r.w.s.Project(RolePlayer),
	}
	ch := r.w.change(cmd, actor)
	r.visit(stepRecord{name: name, actor: actor, hub: hub, change: ch, before: r.current})
}
func (r *recorder) uncovered() []string {
	var missing []string
	for wire := range WireCommandPrototypes() {
		if !r.ran[wire] {
			missing = append(missing, wire)
		}
	}
	for wire := range HubCommandPrototypes() {
		if !r.ran[wire] {
			missing = append(missing, wire)
		}
	}
	return missing
}
func commandWire(cmd Command) string {
	for wire, proto := range WireCommandPrototypes() {
		if sameType(proto, cmd) {
			return wire
		}
	}
	for wire, proto := range HubCommandPrototypes() {
		if sameType(proto, cmd) {
			return wire
		}
	}
	return ""
}
func sameType(a, b Command) bool {
	return typeName(a) == typeName(b)
}
