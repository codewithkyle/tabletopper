package hub
import (
	"reflect"
	"testing"
	"tabletopper/internal/room"
)
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
