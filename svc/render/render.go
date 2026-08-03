// Package render turns markdown bytes into HTML at request time.
package render

import (
	"bytes"
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

const (
	// PRIORITY_TRANSFORM_ALERT and friends order the AST transformers. Lower
	// numbers run first; alerts and mermaid must both run before rendering.
	PRIORITY_TRANSFORM_ALERT   = 900
	PRIORITY_TRANSFORM_MERMAID = 901
	PRIORITY_TRANSFORM_LINK    = 902
	PRIORITY_TRANSFORM_TOC     = 903

	// PRIORITY_RENDER_CUSTOM registers the custom node renderers ahead of
	// goldmark's defaults (which sit at 1000).
	PRIORITY_RENDER_CUSTOM = 100
)

// Document is one rendered markdown file.
type Document struct {
	HTML        template.HTML
	Title       string
	Description string
	Meta        map[string]any
	Headings    []Heading
	HasMermaid  bool
}

// Renderer is safe for concurrent use.
type Renderer struct {
	md goldmark.Markdown
}

// New builds the markdown pipeline: GitHub-flavoured markdown, YAML
// frontmatter, GitHub alerts, mermaid fences and chroma syntax highlighting.
func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.DefinitionList,
			extension.Typographer,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle(CHROMA_STYLE_LIGHT),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithLineNumbers(false),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
			parser.WithASTTransformers(
				util.Prioritized(alertTransformer{}, PRIORITY_TRANSFORM_ALERT),
				util.Prioritized(mermaidTransformer{}, PRIORITY_TRANSFORM_MERMAID),
				util.Prioritized(linkTransformer{}, PRIORITY_TRANSFORM_LINK),
				util.Prioritized(tocTransformer{}, PRIORITY_TRANSFORM_TOC),
			),
		),
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(customRenderer{}, PRIORITY_RENDER_CUSTOM),
			),
		),
	)
	return &Renderer{md: md}
}

// Render converts source into a Document. Frontmatter drives Title and
// Description; both fall back to the document body when absent.
func (r *Renderer) Render(source []byte) (*Document, error) {
	pc := parser.NewContext()
	var buf bytes.Buffer
	if err := r.md.Convert(source, &buf, parser.WithContext(pc)); err != nil {
		return nil, err
	}

	doc := &Document{
		HTML:       template.HTML(buf.String()),
		Meta:       meta.Get(pc),
		Headings:   collectedHeadings(pc),
		HasMermaid: hasMermaid(pc),
	}
	doc.Title = metaString(doc.Meta, "title")
	doc.Description = metaString(doc.Meta, "description")
	if doc.Title == "" {
		doc.Title = firstHeadingText(doc.Headings)
	}
	return doc, nil
}

// firstHeadingText falls back to the document's opening heading when
// frontmatter carries no title.
func firstHeadingText(headings []Heading) string {
	if len(headings) == 0 {
		return ""
	}
	return headings[0].Text
}

// customRenderer emits the node kinds this package adds to the AST.
type customRenderer struct{}

func (c customRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KIND_ALERT, renderAlert)
	reg.Register(KIND_MERMAID, renderMermaid)
}

func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// inlineText flattens a node's inline children to plain text.
func inlineText(n ast.Node, source []byte) string {
	var sb strings.Builder
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := child.(type) {
		case *ast.Text:
			sb.Write(t.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				sb.WriteByte(' ')
			}
		case *ast.String:
			sb.Write(t.Value)
		case *ast.AutoLink:
			sb.Write(t.URL(source))
		}
		return ast.WalkContinue, nil
	})
	return sb.String()
}
