package room

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeCommandReadsAWellFormedFrame(t *testing.T) {
	frame := `{"type":"pawn.move","cid":"a9","anchor":"` + testID(7).String() + `","x":128,"y":64,"others":[]}`
	cmd, cid, err := DecodeCommand([]byte(frame))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cid != "a9" {
		t.Fatalf("cid = %q, want a9", cid)
	}
	move, ok := cmd.(*PawnMove)
	if !ok {
		t.Fatalf("decoded a %T, want a *PawnMove", cmd)
	}
	if move.Anchor != testID(7) || move.X != 128 || move.Y != 64 {
		t.Fatalf("decoded %+v", move)
	}
}
func TestDecodeCommandRefusesEverythingItShould(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"a type nobody has heard of", `{"type":"pawn.teleport","cid":"1"}`},
		{"no type at all", `{"cid":"1"}`},
		{"a field the command does not have", `{"type":"pawn.move","cid":"1","anchor":"` + testID(7).String() + `","speed":9}`},
		{"a ULID that is not one", `{"type":"pawn.move","cid":"1","anchor":"not-a-ulid"}`},
		{"a number where an object belongs", `{"type":"table.setGrid","cid":"1","grid":5}`},
		{"broken JSON", `{"type":`},
		{"a resolved field a client tried to fill in", `{"type":"pawn.spawn","cid":"1","kind":"monster","layer":"` + testID(1).String() + `","x":0,"y":0,"visible":true,"pawn":{}}`},
		{"a hub-only command sent from a browser", `{"type":"player.join","cid":"1","player":{"id":"` + testID(2).String() + `","role":"gm"}}`},
		{"the hub's own room close", `{"type":"room.close","cid":"1"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _, err := DecodeCommand([]byte(tc.frame))
			if err == nil {
				t.Fatalf("decoded %T from %s", cmd, tc.frame)
			}
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("got a %T, want a *room.Error: %v", err, err)
			}
			if e.Code != CodeInvalid {
				t.Fatalf("code = %s, want %s", e.Code, CodeInvalid)
			}
		})
	}
}
func TestEveryWireCommandDecodesUnderItsOwnName(t *testing.T) {
	seen := map[string]string{}
	for wire := range WireCommandPrototypes() {
		cmd, _, err := DecodeCommand([]byte(`{"type":"` + wire + `","cid":"1"}`))
		if err != nil {
			if e, ok := err.(*Error); ok && strings.Contains(e.Message, "does not know") {
				t.Fatalf("%s is in the registry but DecodeCommand does not know it", wire)
			}
			continue
		}
		name := typeName(cmd)
		if other, clash := seen[name]; clash {
			t.Fatalf("%s and %s both decode into a %s", other, wire, name)
		}
		seen[name] = wire
	}
}
func TestEncodeEventPutsTheHeaderFirst(t *testing.T) {
	by := testGMID
	b, err := EncodeEvent(&PawnRemoved{ID: testID(3)}, 4821, &by)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := `{"type":"pawn.removed","seq":4821,"by":"` + by.String() + `","id":"` + testID(3).String() + `"}`
	if string(b) != want {
		t.Fatalf("encoded\n got %s\nwant %s", b, want)
	}
}
func TestEncodeEventOmitsTheActorWhenThereIsNone(t *testing.T) {
	b, err := EncodeEvent(&RoomClosed{}, 12, nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := string(b); got != `{"type":"room.closed","seq":12}` {
		t.Fatalf("encoded %s", got)
	}
}
func TestEveryEventEncodesUnderItsRegisteredType(t *testing.T) {
	for wire, ev := range EventPrototypes() {
		if got := ev.eventType(); got != wire {
			t.Errorf("the event filed under %s calls itself %s", wire, got)
		}
		b, err := EncodeEvent(ev, 1, nil)
		if err != nil {
			t.Fatalf("%s: encode: %v", wire, err)
		}
		var envelope struct {
			Type string `json:"type"`
			Seq  uint64 `json:"seq"`
		}
		if err := json.Unmarshal(b, &envelope); err != nil {
			t.Fatalf("%s: %v", wire, err)
		}
		if envelope.Type != wire {
			t.Errorf("%s encoded with type %q", wire, envelope.Type)
		}
		if envelope.Seq != 1 {
			t.Errorf("%s encoded with seq %d", wire, envelope.Seq)
		}
	}
}
func TestTheTransientEventsAreTheFiveThatAreNotState(t *testing.T) {
	want := map[string]bool{
		"room.closed": true, "player.kicked": true,
		"pawn.dragging": true, "pinged": true, "error": true,
	}
	for wire, ev := range EventPrototypes() {
		if ev.Transient() != want[wire] {
			t.Errorf("%s reports Transient() = %v", wire, ev.Transient())
		}
	}
}
func TestTheTwoRegistriesDoNotOverlap(t *testing.T) {
	wire := WireCommandPrototypes()
	for name := range HubCommandPrototypes() {
		if _, both := wire[name]; both {
			t.Fatalf("%s is in both the wire and the hub registries", name)
		}
	}
}
func TestTheErrorEventCarriesTheRefusal(t *testing.T) {
	ev := NewErrorEvent("a9", forbidden("Not your pawn", "Only the GM can move that."))
	if ev.CID != "a9" || ev.Code != CodeForbidden || ev.Heading != "Not your pawn" {
		t.Fatalf("built %+v", ev)
	}
	if !ev.Transient() {
		t.Fatal("an error is not marked transient, so it would be reduced into state")
	}
	generic := NewErrorEvent("a9", errString("the database is on fire"))
	if generic.Code != CodeInvalid {
		t.Fatalf("a plain error became %s", generic.Code)
	}
	if strings.Contains(generic.Message, "fire") {
		t.Fatalf("the internal message reached the client: %q", generic.Message)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
func typeName(v any) string       { return fmt.Sprintf("%T", v) }
