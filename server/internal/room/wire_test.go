package room

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
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
	b, err := EncodeEvent(&Pinged{Layer: testID(3), X: 64, Y: 32}, 4821, &by)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := `{"type":"pinged","seq":4821,"by":"` + by.String() + `","layer":"` + testID(3).String() + `","x":64,"y":32}`
	if string(b) != want {
		t.Fatalf("encoded\n got %s\nwant %s", b, want)
	}
}
func TestAFrameCarriesEveryDerivedEventOfOneCommand(t *testing.T) {
	by := testGMID
	changes := []Change{
		&PawnsRemoved{IDs: []ulid.ULID{testID(3)}},
		&PawnsMoved{Pawns: []PawnPosition{{ID: testID(4), X: 64, Y: 32}}},
		&TableUpdated{Table: TableSettings{ActiveLayer: testID(5), PawnLabels: LabelsFull}},
	}
	b, err := EncodeEvent(NewChanges(changes), 41, &by)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.HasPrefix(string(b), `{"type":"changes","seq":41,"by":"`+by.String()+`","events":[`) {
		t.Fatalf("the frame does not lead with its own header: %s", b)
	}
	if !strings.Contains(string(b), `{"type":"pawns.removed","ids":`) {
		t.Fatalf("an event inside the frame is not named by its own type: %s", b)
	}
	if strings.Contains(string(b), `"seq":41,"by":"`+by.String()+`","ids"`) {
		t.Fatalf("an event inside the frame carries a sequence of its own: %s", b)
	}
	var back Changes
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Seq != 41 || back.By == nil || *back.By != by {
		t.Fatalf("the header came back as %+v", back.Header)
	}
	if got := changeTypesOf(back.Events); !equalSlices(got, changeTypesOf(changes)) {
		t.Fatalf("the frame came back holding %v, want %v", got, changeTypesOf(changes))
	}
	if got := back.Events[1].(*PawnsMoved).Pawns; len(got) != 1 || got[0].X != 64 {
		t.Fatalf("a decoded event lost its payload: %+v", got)
	}
}
func TestAFrameRefusesAnEventNobodyWrote(t *testing.T) {
	var back Changes
	err := json.Unmarshal([]byte(`{"type":"changes","seq":1,"events":[{"type":"pawns.teleported"}]}`), &back)
	if err == nil {
		t.Fatal("a frame carrying an unknown event was decoded")
	}
	if !strings.Contains(err.Error(), "pawns.teleported") {
		t.Fatalf("the error does not name the event: %v", err)
	}
}
func TestEveryChangeEncodesUnderItsRegisteredType(t *testing.T) {
	for wire, ch := range ChangePrototypes() {
		if got := ch.changeType(); got != wire {
			t.Errorf("the change filed under %s calls itself %s", wire, got)
		}
		b, err := EncodeEvent(NewChanges([]Change{ch}), 1, nil)
		if err != nil {
			t.Fatalf("%s: encode: %v", wire, err)
		}
		if !strings.Contains(string(b), `{"type":"`+wire+`"`) {
			t.Errorf("%s encoded as %s", wire, b)
		}
	}
}
func equalSlices(a, b []string) bool {
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
func TestEveryFrameIsTheChangesFrameOrATransient(t *testing.T) {
	want := map[string]bool{
		"room.closed": true, "player.kicked": true, "snapshot": true,
		"pawn.dragging": true, "pinged": true, "error": true, "rolled": true,
	}
	for wire, ev := range EventPrototypes() {
		_, marked := ev.(Transient)
		if marked != want[wire] && wire != "changes" {
			t.Errorf("%s is a room.Transient = %v", wire, marked)
		}
	}
	if _, ok := EventPrototypes()["changes"]; !ok {
		t.Fatal("the changes frame is not in the event registry, so the client has no type for it")
	}
	if _, marked := EventPrototypes()["changes"].(Transient); marked {
		t.Fatal("the changes frame is marked transient, so it would not advance the sequence")
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
	if _, marked := any(ev).(Transient); !marked {
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
