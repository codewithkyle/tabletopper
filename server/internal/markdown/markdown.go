// Package markdown renders a stored journal body to HTML. It is the read half
// of the editor: journal-editor.js turns markdown into a document in the
// writer's browser, and this turns the same markdown into a document on the
// server for everyone who is not the writer.
//
// NOTHING THAT ARRIVES IN A BODY IS TRUSTED. A body is whatever was in the
// textarea when a save fired, which is whatever the writer typed, pasted, or
// left behind after JavaScript failed to load -- so it may hold raw HTML, a
// javascript: link, or an <img> pointing anywhere. Rendering it for a stranger
// turns anything that survives into that stranger's problem, which is why the
// three defences below are goldmark's defaults rather than options this package
// switched on:
//
//   - Raw HTML is not rendered. goldmark writes `<!-- raw HTML omitted -->` in
//     its place unless html.WithUnsafe() is passed, and it is not passed here
//     or anywhere else. That is the whole of the XSS answer, and it is why no
//     sanitiser is vendored: there is no HTML to sanitise, because none is
//     produced.
//   - javascript:, vbscript:, file: and data: destinations are dropped from
//     links, by the same flag -- see html.IsDangerousURL.
//   - Every text run is HTML-escaped on the way out.
//
// The one thing that is not a default is images, and they are handled here
// because the answer is not "escape it" but "is this picture ours?". See
// ImageSource.
package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ImageSource decides what one image in a body renders as. It is handed the
// destination the markdown carries and returns the URL to serve the picture
// from, or false to drop the image from the document entirely.
//
// FALSE IS THE ANSWER FOR EVERY FOREIGN URL, and dropping rather than rewriting
// is deliberate. An entry can only reference an image this app stored for that
// entry; anything else got there by being typed into the textarea fallback or
// pasted as HTML the editor did not intercept. Left in, it would be a request
// to a third party made by every reader of a shared page, telling that third
// party who was reading and when. The journal entry page already refuses to
// load them with a CSP, and this is the same refusal made where the markup is
// produced instead of where it is displayed.
//
// The rewrite half is what a share needs. An image's stored URL names the
// owner's own route, which a signed-out reader cannot fetch, so the share
// render maps it onto a URL scoped to the share -- and a destination that does
// not belong to the entry being rendered has nothing to map onto and is
// dropped by the same rule.
type ImageSource func(dest string) (string, bool)

// imagesKey carries the caller's ImageSource from Render into the transformer.
// goldmark.Markdown is built once and used from every request, so the hook
// cannot be a field on it; the parser context is per-parse and is what the
// transformer is handed.
var imagesKey = parser.NewContextKey()

// md is the whole renderer, built once. Convert is safe to call concurrently --
// the parse state it needs lives in the context passed to each call.
//
// STRIKETHROUGH IS THE ONE EXTENSION, because it is the one mark the editor can
// produce that CommonMark cannot express: StarterKit ships strike and
// serialises it as ~~text~~, which without this renders as literal tildes.
// Tables, task lists and autolinks are the rest of GFM and none of them is in
// the toolbar, so parsing them here would render something the editor cannot
// round-trip.
var md = goldmark.New(
	goldmark.WithExtensions(extension.Strikethrough),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(imageTransformer{}, 100)),
	),
)

// Render turns one journal body into HTML, asking images about every picture in
// it. The result is trusted markup by construction -- see the package comment
// for what that rests on -- and callers write it with templ.Raw.
//
// An empty body renders as an empty string rather than an error: an entry is
// born blank and stays that way until someone writes in it.
func Render(body string, images ImageSource) (string, error) {
	ctx := parser.NewContext()
	ctx.Set(imagesKey, images)

	var out bytes.Buffer
	if err := md.Convert([]byte(body), &out, parser.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("markdown: render: %w", err)
	}

	return out.String(), nil
}

// imageTransformer applies the caller's ImageSource to the parsed document,
// before rendering rather than during it. Replacing goldmark's image renderer
// would mean reproducing its alt-text and title handling to change one
// attribute; rewriting the node instead leaves all of that where it is and
// lets the default renderer escape the destination the way it escapes every
// other one.
type imageTransformer struct{}

func (imageTransformer) Transform(doc *ast.Document, _ text.Reader, ctx parser.Context) {
	images, _ := ctx.Get(imagesKey).(ImageSource)
	if images == nil {
		return
	}

	// Collected first and mutated after, because removing a node during a walk
	// cuts the walk's own link to the rest of the tree.
	var found []*ast.Image
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if image, ok := node.(*ast.Image); ok && entering {
			found = append(found, image)
		}

		return ast.WalkContinue, nil
	})

	for _, image := range found {
		dest, keep := images(string(image.Destination))
		if keep {
			image.Destination = []byte(dest)
			continue
		}

		parent := image.Parent()
		parent.RemoveChild(parent, image)
		// A picture on a line of its own is still a paragraph containing one
		// image, so dropping it leaves an empty <p> in the middle of the prose
		// with the margin of a real one. Nothing else in a body can produce an
		// empty paragraph, so an empty one here is always this.
		if parent.Kind() == ast.KindParagraph && !parent.HasChildren() {
			if above := parent.Parent(); above != nil {
				above.RemoveChild(above, parent)
			}
		}
	}
}

// PlainText projects one journal body onto the words a reader actually sees. It
// is what the search snippets are cut from, and what decides a search result is
// real -- see internal/snippet.
//
// IT IS THE SAME PARSE Render RUNS, not a pass of regular expressions over the
// source. The difference is the whole point: a link is `[Thistlewick](/assets/
// images/01J...)` in the markdown and the word `Thistlewick` on the page, so a
// projection that does not understand the syntax would report the storage URL
// as something the writer wrote. Structure is dropped by taking only the text
// nodes -- heading markers, list bullets, emphasis marks and table pipes are
// never text nodes to begin with, so none of them has to be recognised and
// stripped.
//
// WHAT IS LEFT OUT IS WHAT IS NOT READ. Link and image destinations are
// attributes rather than children and do not survive; raw HTML does not either,
// which matches the renderer dropping it. Alt text and code blocks do survive:
// both are words somebody typed on purpose, and a term found only in a code
// block should still find its entry.
//
// Runs of whitespace collapse to one space, so a phrase written across a soft
// line break is still one phrase to search -- and so a snippet is a line rather
// than a piece of a paragraph's shape.
//
// The transformer Render relies on reads its ImageSource out of the parse
// context and does nothing without one, so parsing here leaves images alone.
func PlainText(body string) string {
	source := []byte(body)
	doc := md.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))

	var out strings.Builder
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if node.Type() == ast.TypeBlock {
				out.WriteByte('\n')
			}

			return ast.WalkContinue, nil
		}

		switch node := node.(type) {
		case *ast.Text:
			out.Write(node.Segment.Value(source))
			if node.SoftLineBreak() || node.HardLineBreak() {
				out.WriteByte('\n')
			}
		case *ast.String:
			out.Write(node.Value)
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			lines := node.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				out.Write(line.Value(source))
			}
		}

		return ast.WalkContinue, nil
	})

	return strings.Join(strings.Fields(out.String()), " ")
}
