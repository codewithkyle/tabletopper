package main
import (
	"os"
	"strings"
	"testing"
)
const committed = "../../../js/room/protocol.ts"
func TestProtocolTypesAreCurrent(t *testing.T) {
	got, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want, err := os.ReadFile(committed)
	if err != nil {
		t.Fatalf("%v -- run `go generate ./...` from ./server", err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale. Run `go generate ./...` from ./server and commit the result.", committed)
	}
}
func TestTheGeneratedFileIsADiscriminatedUnion(t *testing.T) {
	b, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	out := string(b)
	for _, want := range []string{
		`export type Command =`,
		`export type Event =`,
		`export const TRANSIENT_EVENTS: ReadonlySet<Event["type"]> = new Set([`,
		`	type: "pawn.move";`,
		`	type: "pawn.moved";`,
		`	cid: string;`,
		`	seq: number;`,
		`	by?: string;`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the generated file does not contain %q", want)
		}
	}
	if !strings.Contains(out, `export type Size = "tiny" | "small" | "medium" | "large" | "huge" | "gargantuan";`) {
		t.Error("the size enum was not emitted as a literal union")
	}
	if !strings.Contains(out, "\tanchor: string;") {
		t.Error("a ULID field was not emitted as a string")
	}
	if strings.Contains(out, "\tpawn: Pawn;\n\tcid") || strings.Contains(out, "\tmap: MapRef | null;\n\tcid") {
		t.Error("a resolved field reached the generated client types")
	}
	for _, hidden := range []string{`"player.join"`, `"player.setConnected"`, `"room.setLocked"`, `"room.close"`} {
		if strings.Contains(out, hidden) {
			t.Errorf("the hub-only command %s was emitted to the client", hidden)
		}
	}
}
