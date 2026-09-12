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

type generator struct {
	enums   map[string]reflect.Type
	structs map[string]reflect.Type
	queue   []reflect.Type
}

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
	g.want(reflect.TypeOf(room.State{}))
	entities := map[string]string{}
	for len(g.queue) > 0 {
		t := g.queue[0]
		g.queue = g.queue[1:]
		entities[t.Name()] = g.iface(t.Name(), g.tsFields(t))
	}
	var b bytes.Buffer
	for _, name := range sortedKeys(g.enums) {
		b.WriteString(g.enum(name, g.enums[name]))
	}
	for _, name := range sortedKeys(entities) {
		b.WriteString(entities[name])
	}
	writeBlocks(&b, commands)
	b.WriteString(union("Command", commands))
	writeBlocks(&b, events)
	b.WriteString(union("Event", events))
	b.WriteString(transient())
	return b.Bytes(), nil
}

type block struct {
	Wire string
	Name string
	Body string
}

func (g *generator) render(from map[string]any, envelope func(name string) []tsField) []block {
	out := make([]block, 0, len(from))
	for _, wire := range sortedKeys(from) {
		t := reflect.TypeOf(from[wire]).Elem()
		fields := append(envelope(wire), g.tsFields(t)...)
		out = append(out, block{Wire: wire, Name: t.Name(), Body: g.iface(t.Name(), fields)})
	}
	return out
}

type tsField struct {
	Name     string
	Type     string
	Optional bool
}

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
func (g *generator) tsType(t reflect.Type) string {
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
func transient() string {
	var names []string
	for wire, ev := range room.EventPrototypes() {
		if _, ok := ev.(room.Transient); ok {
			names = append(names, quote(wire))
		}
	}
	slices.Sort(names)
	var b strings.Builder
	b.WriteString("export const TRANSIENT_EVENTS: ReadonlySet<Event[\"type\"]> = new Set([\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\t%s,\n", n)
	}
	b.WriteString("]);\n")
	return b.String()
}
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
func prototypeMap[T any](m map[string]T) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
