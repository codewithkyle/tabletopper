package hub

import (
	"reflect"
	"testing"

	"tabletopper/internal/room"
)

// THE CONTRACT resolve.go ENFORCES BY HAND, CHECKED BY REFLECTION. A command
// whose struct carries a field the wire never sets -- `json:"-"` -- is a command
// the hub has to fill in before it reaches the room, and the only thing that
// says which commands those are is a type switch. A command added with such a
// field and forgotten there would reach Apply with the field nil and be refused
// at a table with a message about the wrong thing. This walks every wire
// command, finds the ones with such a field, and asserts that resolve does
// something about each of them.
//
// WITH NO DATABASE, "something" is the not-built refusal, which is what a hub
// built with nil queries answers for every resolver it has. A command that
// resolve does not know comes back with no error, which is the failure.
//
// AND A PLAYER COSTS NOTHING. Every resolved command is the GM's, so for
// anybody else resolve returns before it would have read a row; the field is
// left nil for Authorize to refuse on. That is the ordering S6 in the review
// asked for, pinned.
func TestEveryCommandWithAResolvedFieldIsResolvedAndOnlyForTheGM(t *testing.T) {
	h := New(nil, Options{Store: &memStore{}})
	gm := room.Actor{ID: gmID, Role: room.RoleGM}
	player := room.Actor{ID: playerID, Role: room.RolePlayer}

	found := 0
	for name, cmd := range room.WireCommandPrototypes() {
		fields := resolvedFields(cmd)
		if len(fields) == 0 {
			continue
		}
		found++

		if err := h.resolve(t.Context(), roomID, gm, cmd); err == nil {
			t.Errorf("%s carries %v but resolve did nothing with it for the GM", name, fields)
		}

		fresh := room.WireCommandPrototypes()[name]
		if err := h.resolve(t.Context(), roomID, player, fresh); err != nil {
			t.Errorf("%s: resolve did work for a player, who may never send it: %v", name, err)
		}
		for _, field := range fields {
			if !reflect.ValueOf(fresh).Elem().FieldByName(field).IsNil() {
				t.Errorf("%s: resolve filled %s in for a player", name, field)
			}
		}
	}

	if found == 0 {
		t.Fatal("no command carries a resolved field; the test is looking at the wrong thing")
	}
}

// resolvedFields is the names of the struct's fields tagged `json:"-"`.
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
