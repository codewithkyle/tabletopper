package main

import (
	"os"
	"strings"
	"testing"
)

// committed is the same file the output constant names, from here rather than
// from internal/room. The two paths differ because go:generate runs the
// generator with the generating package's directory as its working directory
// and `go test` runs this one with its own.
const committed = "../../../js/room/protocol.ts"

// A STALE protocol.ts BREAKS THE BUILD RATHER THAN A TABLE. The whole point of
// generating the client's types is that they cannot disagree with the server's,
// and a generated file that is only regenerated when somebody remembers is a
// hand-maintained file with extra steps.
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

// The two things a hand-written client would get wrong, pinned here because
// they are what the discriminated unions are for: every command and event has a
// literal type, and the unions name every one of them.
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

	// A ULID is its string everywhere but Go, and an enum is a literal union
	// rather than a bare string.
	if !strings.Contains(out, `export type Size = "tiny" | "small" | "medium" | "large" | "huge" | "gargantuan";`) {
		t.Error("the size enum was not emitted as a literal union")
	}
	if !strings.Contains(out, "\tanchor: string;") {
		t.Error("a ULID field was not emitted as a string")
	}

	// THE RESOLVED FIELDS ARE NOT THERE. A client with a type for pawn.spawn's
	// resolved Pawn would think it was supposed to send one, and the whole
	// point of resolving on the server is that it does not.
	if strings.Contains(out, "\tpawn: Pawn;\n\tcid") || strings.Contains(out, "\tmap: MapRef | null;\n\tcid") {
		t.Error("a resolved field reached the generated client types")
	}

	// AND NEITHER ARE THE HUB-ONLY COMMANDS. A client that had a type for
	// player.join would look like a client that could send one.
	for _, hidden := range []string{`"player.join"`, `"player.setConnected"`, `"room.setLocked"`, `"room.close"`} {
		if strings.Contains(out, hidden) {
			t.Errorf("the hub-only command %s was emitted to the client", hidden)
		}
	}
}
