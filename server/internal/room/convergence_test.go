package room

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/oklog/ulid/v2"
)

// -update rewrites the golden fixtures instead of comparing against them.
//
//	go test ./internal/room -update
var update = flag.Bool("update", false, "rewrite the reducer fixtures in testdata")

// THE CONVERGENCE PROPERTY, which is the one this protocol was designed around
// and the reason full-entity events are worth their bytes:
//
//	reduce(project(before), events for that audience) == project(after)
//
// If that holds for every command and both audiences, a client that starts from
// a snapshot and applies every event afterwards is holding exactly what the
// server holds, and it holds it without any partial-update semantics, any
// null-versus-absent question, or any per-field merge.
//
// THE OLD PROTOCOL'S EIGHT PER-FIELD PAWN EVENTS made this something to hope
// for. Here it is something that fails a build.
//
// IT IS CHECKED PER COMMAND rather than at the end of the run, so a failure
// names the command that broke it rather than the fifty-step scenario that
// ended up wrong.
func TestEmittedEventsConvergeOnTheServersState(t *testing.T) {
	w := newWorld(t)

	r := &recorder{t: t, w: w}
	r.visit = func(st stepRecord) {
		// EVERY APPLY LEAVES THE STATE NORMALIZED. It is the precondition the
		// byte comparisons below rest on, and Clone normalizes on its way out,
		// so a state that is already normalized is its own clone.
		if got, want := mustJSON(t, w.s), mustJSON(t, w.s.Clone()); got != want {
			t.Fatalf("%s: the state it left behind is not normalized\n  got %s\n want %s", st.name, got, want)
		}

		for _, v := range r.viewers() {
			got := v.before.Clone()

			for _, ev := range delivered(st.emissions, st.actor, v.actor) {
				if err := Reduce(&got, ev); err != nil {
					t.Fatalf("%s: reducing %s for the %s: %v", st.name, ev.eventType(), v.actor.Role, err)
				}
			}

			want := w.s.Project(v.actor.Role)
			if mustJSON(t, got) != mustJSON(t, want) {
				t.Fatalf("%s: the %s's reduced state is not the server's\n reduced %s\n  server %s",
					st.name, v.actor.Role, mustJSON(t, got), mustJSON(t, want))
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

// THE GOLDEN FIXTURES. The same scenario, written out as the events one
// audience receives and the state they should be holding afterwards, so that
// phase 3's TypeScript reducer is tested against this one rather than against a
// second reading of the specification.
//
// Regenerate with `go test ./internal/room -update` and read the diff: a change
// here is a change to what every client will do, and it should be one somebody
// meant.
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
				events := []json.RawMessage{}

				for _, ev := range delivered(st.emissions, st.actor, viewer) {
					seq++

					// The sequence is stamped here rather than by Apply,
					// because assigning it is the hub's job in phase 3 and
					// nothing in this package knows how many events went before.
					var by *ulid.ULID
					if !st.hub {
						id := st.actor.ID
						by = &id
					}

					b, err := EncodeEvent(ev, seq, by)
					if err != nil {
						t.Fatalf("%s: encode: %v", st.name, err)
					}
					events = append(events, json.RawMessage(b))
				}

				fixture.Steps = append(fixture.Steps, reducerStep{
					Name:   st.name,
					Events: events,
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

// A fixture replays end to end, not only step by step. The convergence test
// checks each command against the server's own state; this checks that a client
// which starts from the initial snapshot and never resyncs ends up in the same
// place.
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
				for _, raw := range step.Events {
					ev, err := decodeEventForTest(raw)
					if err != nil {
						t.Fatalf("%s: %v", step.Name, err)
					}
					if err := Reduce(&state, ev); err != nil {
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
	Events []json.RawMessage `json:"events"`
	State  State             `json:"state"`
}

// decodeEventForTest turns a recorded frame back into an event. There is no
// decoder for events in the package itself and there should not be -- the
// server writes events and never reads them -- so this exists for the replay
// test alone, built out of the same registry the generator emits from.
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

// stepRecord is one command's worth of what the two tests above need.
type stepRecord struct {
	name      string
	actor     Actor
	hub       bool
	emissions []Emission
	before    map[Role]State
}

// recorder drives the scenario and hands each step to whichever of the two
// tests is running it. The script is written once, in scenario below, because a
// scenario written twice is two scenarios that drift.
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

// do runs one command from a person, records what it emitted, and answers with
// the emissions so the script can pull a new id out of them.
func (r *recorder) do(name string, cmd Command, actor Actor) []Emission {
	r.t.Helper()

	return r.record(name, cmd, actor, false)
}

// hub runs one of the commands only the server builds. They carry no `by`,
// because nobody sent them.
func (r *recorder) hub(name string, cmd Command) []Emission {
	r.t.Helper()

	return r.record(name, cmd, r.w.gm, true)
}

func (r *recorder) record(name string, cmd Command, actor Actor, hub bool) []Emission {
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

	ems := r.w.apply(cmd, actor)

	r.visit(stepRecord{name: name, actor: actor, hub: hub, emissions: ems, before: r.current})

	return ems
}

// uncovered names any command the scenario never runs, so that adding one to
// the registry without adding it to the script fails rather than quietly going
// untested.
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

// commandWire finds a command's type string by looking it up in the registries
// it came from, which is the same lookup the hub does in reverse.
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
