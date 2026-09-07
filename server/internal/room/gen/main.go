// Command gen writes the TypeScript mirror of the room protocol.
//
// GO OWNS THE TYPES AND GENERATES THE TYPESCRIPT, which is the wire-format
// decision the architecture overview makes and this is the whole of its
// implementation. The server validates every message, so its structs are the
// authority on what a message is; a second, hand-written set of types on the
// client would agree until somebody changed one of them, and the disagreement
// would be found at a table rather than in a build.
//
// TYGO WAS CONSIDERED AND REJECTED. It emits interfaces from structs perfectly
// well, and it cannot emit the two discriminated unions, because those are
// built from the type strings in the command and event registries rather than
// from anything in the struct definitions. The registries are the one place
// those strings already live, so the generator reads them -- and once it is
// reading a registry, the rest of the reflection is a hundred lines it now
// fully controls.
//
// Run it with `go generate ./...` from ./server. TestProtocolTypesAreCurrent
// regenerates into a buffer and fails when the committed file disagrees, so a
// stale protocol.ts breaks `make check` rather than a browser.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

// output is relative to internal/room, which is where go:generate runs this.
const output = "../../js/room/protocol.ts"

func main() {
	b, err := Generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(output, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

var (
	ulidType   = reflect.TypeOf(ulid.ULID{})
	headerType = reflect.TypeOf(room.Header{})
	valuesType = reflect.TypeOf((*interface{ Values() []string })(nil)).Elem()
)

// generator collects what it has to emit while it walks. Everything it holds is
// keyed by TypeScript name and emitted in sorted order, so the file is a
// function of the Go types and not of map iteration.
type generator struct {
	enums   map[string]reflect.Type
	structs map[string]reflect.Type
	queue   []reflect.Type
}

// Generate produces the whole file.
func Generate() ([]byte, error) {
	g := &generator{
		enums:   map[string]reflect.Type{},
		structs: map[string]reflect.Type{},
	}

	commands := g.render(prototypeMap(room.WireCommandPrototypes()), func(name string) []tsField {
		return []tsField{
			{Name: "type", Type: quote(name)},
			{Name: "cid", Type: "string"},
		}
	})

	events := g.render(prototypeMap(room.EventPrototypes()), func(name string) []tsField {
		return []tsField{
			{Name: "type", Type: quote(name)},
			{Name: "seq", Type: "number"},
			{Name: "by", Type: "string", Optional: true},
		}
	})

	// State is not reachable from a command and is reachable from exactly one
	// event, so it is asked for by name rather than found. It is also the type
	// the client's reducer is written against, which makes it the reason this
	// file exists at all.
	g.want(reflect.TypeOf(room.State{}))

	entities := map[string]string{}
	for len(g.queue) > 0 {
		t := g.queue[0]
		g.queue = g.queue[1:]
		entities[t.Name()] = g.iface(t.Name(), g.tsFields(t))
	}

	var b bytes.Buffer
	b.WriteString(preamble)

	b.WriteString("\n// The enumerations. Each one is generated from that type's own Values method\n")
	b.WriteString("// in Go, which is the same list the server validates against.\n\n")
	for _, name := range sortedKeys(g.enums) {
		b.WriteString(g.enum(name, g.enums[name]))
		b.WriteString("\n")
	}

	b.WriteString("\n// The entities. These are the objects that appear inside a snapshot and\n")
	b.WriteString("// inside the events that carry a whole entity.\n\n")
	for _, name := range sortedKeys(entities) {
		b.WriteString(entities[name])
		b.WriteString("\n")
	}

	b.WriteString("\n// The commands: what a client sends. Every one carries a client correlation\n")
	b.WriteString("// id, which comes back on an error so the client knows which of its\n")
	b.WriteString("// outstanding requests was refused.\n\n")
	writeBlocks(&b, commands)
	b.WriteString(union("Command", commands))

	b.WriteString("\n// The events: what the server broadcasts, in the past tense. An event is the\n")
	b.WriteString("// server's version of what happened, which may differ from what was asked --\n")
	b.WriteString("// snapped to the grid, resolved against the monster manual, or projected down\n")
	b.WriteString("// to less than the commanding client can see.\n\n")
	writeBlocks(&b, events)
	b.WriteString(union("Event", events))

	b.WriteString(transient())

	return b.Bytes(), nil
}

// block is one emitted interface and the wire type string it was filed under,
// kept together so the unions below can be built in the same order.
type block struct {
	Wire string
	Name string
	Body string
}

// render turns a registry into interfaces. envelope supplies the fields that
// belong to the frame rather than to the operation -- type and cid for a
// command, type and seq and by for an event -- because those are not fields of
// any Go struct and should not be.
func (g *generator) render(from map[string]any, envelope func(name string) []tsField) []block {
	out := make([]block, 0, len(from))

	for _, wire := range sortedKeys(from) {
		t := reflect.TypeOf(from[wire]).Elem()
		fields := append(envelope(wire), g.tsFields(t)...)
		out = append(out, block{Wire: wire, Name: t.Name(), Body: g.iface(t.Name(), fields)})
	}

	return out
}

// tsField is one line of an interface.
type tsField struct {
	Name     string
	Type     string
	Optional bool
}

// tsFields flattens one struct into its TypeScript lines.
//
// IT DROPS THREE KINDS OF FIELD, each for its own reason. An unexported field
// is not on the wire at all -- the pre-projected player copies the two hot-path
// events carry are unexported precisely so that they cannot be. A field tagged
// json:"-" is a resolved field, filled in by the hub from the database after
// the client's message arrived, so a client that had a type for it would think
// it was supposed to send one. And the embedded Header is replaced by the
// envelope above, which knows the literal type string this event always has.
func (g *generator) tsFields(t reflect.Type) []tsField {
	var out []tsField

	for i := range t.NumField() {
		f := t.Field(i)

		if f.Anonymous && f.Type == headerType {
			continue
		}
		if f.PkgPath != "" {
			continue
		}

		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}

		if f.Anonymous && f.Type.Kind() == reflect.Struct && tag == "" {
			out = append(out, g.tsFields(f.Type)...)

			continue
		}

		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" {
			name = f.Name
		}

		out = append(out, tsField{
			Name:     name,
			Type:     g.tsType(f.Type),
			Optional: slices.Contains(parts[1:], "omitempty"),
		})
	}

	return out
}

// tsType maps one Go type. Registering the named structs and enums it meets is
// a side effect on purpose: the set of types to emit is exactly the set
// reachable from a command, an event or the state, and walking is how that set
// is discovered.
func (g *generator) tsType(t reflect.Type) string {
	// A ULID is sixteen bytes in Go and its 26-character string everywhere
	// else, because it marshals through MarshalText. This has to come before
	// the kind switch, which would otherwise see an array of numbers.
	if t == ulidType {
		return "string"
	}

	if t.Kind() == reflect.String && t.Name() != "" && t.Implements(valuesType) {
		g.enums[t.Name()] = t

		return t.Name()
	}

	switch t.Kind() {
	case reflect.Pointer:
		return g.tsType(t.Elem()) + " | null"

	case reflect.Slice, reflect.Array:
		return wrap(g.tsType(t.Elem())) + "[]"

	case reflect.Map:
		return "Record<string, " + g.tsType(t.Elem()) + ">"

	case reflect.Bool:
		return "boolean"

	case reflect.String:
		return "string"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"

	case reflect.Struct:
		g.want(t)

		return t.Name()
	}

	panic(fmt.Sprintf("gen: no TypeScript for %s", t))
}

// want queues a named struct for emission, once.
func (g *generator) want(t reflect.Type) {
	if t.Name() == "" {
		panic("gen: anonymous struct on the wire")
	}
	if _, seen := g.structs[t.Name()]; seen {
		return
	}

	g.structs[t.Name()] = t
	g.queue = append(g.queue, t)
}

func (g *generator) iface(name string, fields []tsField) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)

	for _, f := range fields {
		optional := ""
		if f.Optional {
			optional = "?"
		}
		fmt.Fprintf(&b, "\t%s%s: %s;\n", f.Name, optional, f.Type)
	}

	b.WriteString("}\n")

	return b.String()
}

func (g *generator) enum(name string, t reflect.Type) string {
	values := reflect.New(t).Elem().Interface().(interface{ Values() []string }).Values()

	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, quote(v))
	}

	return fmt.Sprintf("export type %s = %s;\n", name, strings.Join(quoted, " | "))
}

func writeBlocks(b *bytes.Buffer, blocks []block) {
	for _, bl := range blocks {
		b.WriteString(bl.Body)
		b.WriteString("\n")
	}
}

func union(name string, blocks []block) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export type %s =\n", name)

	for _, bl := range blocks {
		fmt.Fprintf(&b, "\t| %s\n", bl.Name)
	}

	b.WriteString("\t;\n")

	return b.String()
}

// transient emits the set the client splits its dispatch on: an event in it
// never reaches the reducer and is handled by the effects layer instead. It is
// generated from the Go types' own Transient methods rather than written out
// here, so a new transient event cannot be added on one side only.
func transient() string {
	var names []string
	for wire, ev := range room.EventPrototypes() {
		if ev.Transient() {
			names = append(names, quote(wire))
		}
	}
	slices.Sort(names)

	var b strings.Builder
	b.WriteString("\n// The events that never touch the reducer. A ping fades, a drag ghost is\n")
	b.WriteString("// drawn and forgotten, an error opens a dialog: none of them is state, and\n")
	b.WriteString("// none of them is in the snapshot, because there is nothing to restore.\n")
	b.WriteString("export const TRANSIENT_EVENTS: ReadonlySet<Event[\"type\"]> = new Set([\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\t%s,\n", n)
	}
	b.WriteString("]);\n")

	return b.String()
}

// wrap parenthesises a union before an array suffix, so that a nullable element
// type reads as (T | null)[] rather than as T | null[].
func wrap(ts string) string {
	if strings.Contains(ts, " | ") {
		return "(" + ts + ")"
	}

	return ts
}

func quote(s string) string { return `"` + s + `"` }

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)

	return out
}

// prototypeMap widens the event registry to the same shape render takes for
// commands. Go will not convert map[string]room.Event to map[string]any on its
// own, and one helper is less noise than two nearly identical render methods.
func prototypeMap[T any](m map[string]T) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}

	return out
}

const preamble = `// Code generated by internal/room/gen. DO NOT EDIT.
//
// Regenerate from ./server with:
//
//     go generate ./...
//
// and commit the result. TestProtocolTypesAreCurrent fails when this file and
// the Go types disagree, so a stale copy breaks the build rather than a table.
//
// The Go types in internal/room are the authority: the server validates every
// message, so its structs are what a message has to be. Nothing here is
// hand-edited, and nothing here is the place to add a field.
`
