package room

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A frame a generated client would send decodes into the command it names, and
// the correlation id comes back beside it so an error can be paired with the
// request that caused it.
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

// EVERYTHING DECODE REFUSES IS invalid, because every one of them is a client
// that sent something no generated client could produce. A player never sees
// one of these; a developer does, which is why they are separated from
// forbidden.
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

		// THE RESOLVED FIELD IS NOT ON THE WIRE. A client that sent a whole
		// pawn would be describing the thing the hub is supposed to look up,
		// and json:"-" means the key is unknown rather than ignored.
		{"a resolved field a client tried to fill in", `{"type":"pawn.spawn","cid":"1","kind":"monster","layer":"` + testID(1).String() + `","x":0,"y":0,"visible":true,"pawn":{}}`},

		// AND THE HUB-ONLY COMMANDS ARE NOT REACHABLE AT ALL. player.join
		// carries a whole Player with a role in it, so a browser that could
		// send one could make itself the GM.
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

// Every command in the wire registry has to be reachable by the name it is
// filed under, and has to come back as a distinct type. A copy-paste in the
// registry -- two names building the same struct -- is the mistake this finds.
func TestEveryWireCommandDecodesUnderItsOwnName(t *testing.T) {
	seen := map[string]string{}

	for wire := range WireCommandPrototypes() {
		cmd, _, err := DecodeCommand([]byte(`{"type":"` + wire + `","cid":"1"}`))

		// Some commands cannot decode from an empty payload, and that is fine
		// -- what matters is that the name resolved to something rather than to
		// an unknown type.
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

// The header is the first thing in a frame, because reading one in devtools
// should start with what it is.
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

// An event the server caused on its own has no `by`, and omitempty is what
// keeps a null out of every frame that is not a player's doing.
func TestEncodeEventOmitsTheActorWhenThereIsNone(t *testing.T) {
	b, err := EncodeEvent(&RoomClosed{}, 12, nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got := string(b); got != `{"type":"room.closed","seq":12}` {
		t.Fatalf("encoded %s", got)
	}
}

// EncodeEvent stamps the type from the event's own method rather than from a
// literal somebody wrote into the struct, so the two can never disagree. This
// walks the whole catalog to prove the registry key, the method and the encoded
// frame are one string.
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

// The transient set is a property of each event type, and the five that are in
// it are the five the client's effects layer handles instead of its reducer.
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

// Nothing may be in both registries. The whole defence for the hub-only
// commands is that DecodeCommand cannot reach them.
func TestTheTwoRegistriesDoNotOverlap(t *testing.T) {
	wire := WireCommandPrototypes()

	for name := range HubCommandPrototypes() {
		if _, both := wire[name]; both {
			t.Fatalf("%s is in both the wire and the hub registries", name)
		}
	}
}

// An error is built from a refusal and paired with the frame that caused it.
// Anything that is not a *room.Error becomes a generic invalid rather than
// leaking whatever an internal message happened to say.
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

func typeName(v any) string { return fmt.Sprintf("%T", v) }
