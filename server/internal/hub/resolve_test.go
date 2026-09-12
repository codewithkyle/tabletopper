package hub

import (
	"context"
	"reflect"
	"testing"

	"tabletopper/internal/room"
)

func TestEveryCommandWithAResolvedFieldIsAResolver(t *testing.T) {
	found := 0
	for name, cmd := range room.WireCommandPrototypes() {
		fields := resolvedFields(cmd)
		_, resolver := cmd.(room.Resolver)
		if len(fields) > 0 && !resolver {
			t.Errorf("%s carries %v and does not resolve, so nothing will ever fill it in", name, fields)
		}
		if resolver && len(fields) == 0 {
			t.Errorf("%s resolves but carries no field for the answer", name)
		}
		if resolver {
			found++
		}
	}
	if found == 0 {
		t.Fatal("no command resolves; the test is looking at the wrong thing")
	}
}
func resolvedFields(cmd room.Command) []string {
	v := reflect.ValueOf(cmd).Elem()
	var out []string
	for i := range v.NumField() {
		if v.Type().Field(i).Tag.Get("json") == "-" {
			out = append(out, v.Type().Field(i).Name)
		}
	}
	return out
}

type countingResolver struct {
	reads   int
	library bool
	state   bool
}

func (c *countingResolver) Authorize(s *room.State, a room.Actor) error { return nil }
func (c *countingResolver) Apply(s *room.State, a room.Actor, env room.Env) ([]room.Signal, error) {
	return nil, nil
}
func (c *countingResolver) Resolve(ctx context.Context, lib room.Library, s *room.State) error {
	c.reads++
	c.library = lib != nil
	c.state = s != nil && len(s.Table.Layers) > 0
	return nil
}
func TestTheHubResolvesForTheGMAloneAndHandsOverTheRoomAndTheLibrary(t *testing.T) {
	tb := newTabletop(t, Options{})
	cmd := &countingResolver{}
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("the GM's command was refused: %v", err)
	}
	if cmd.reads != 1 {
		t.Fatalf("resolve ran %d times for the GM, want once", cmd.reads)
	}
	if !cmd.library {
		t.Error("resolve was handed no library to read")
	}
	if !cmd.state {
		t.Error("resolve was handed no room to decide against")
	}
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: playerID, Role: room.RolePlayer}, cmd); err != nil {
		t.Fatalf("a player's command was refused by the hub rather than by the command: %v", err)
	}
	if cmd.reads != 1 {
		t.Fatalf("resolve ran %d times, want only the GM's; a player may never ask the GM's library", cmd.reads)
	}
}
func TestACommandThatDoesNotResolveIsDispatchedWithoutOne(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM},
		&room.TableSetActiveLayer{Layer: layer}); err != nil {
		t.Fatalf("a command with nothing to resolve was refused: %v", err)
	}
}
