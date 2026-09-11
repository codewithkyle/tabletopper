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
type ImageSource func(dest string) (string, bool)
var imagesKey = parser.NewContextKey()
var md = goldmark.New(
	goldmark.WithExtensions(extension.Strikethrough),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(imageTransformer{}, 100)),
	),
)
func Render(body string, images ImageSource) (string, error) {
	ctx := parser.NewContext()
	ctx.Set(imagesKey, images)
	var out bytes.Buffer
	if err := md.Convert([]byte(body), &out, parser.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("markdown: render: %w", err)
	}
	return out.String(), nil
}
type imageTransformer struct{}
func (imageTransformer) Transform(doc *ast.Document, _ text.Reader, ctx parser.Context) {
	images, _ := ctx.Get(imagesKey).(ImageSource)
	if images == nil {
		return
	}
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
		if parent.Kind() == ast.KindParagraph && !parent.HasChildren() {
			if above := parent.Parent(); above != nil {
				above.RemoveChild(above, parent)
			}
		}
	}
}
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
