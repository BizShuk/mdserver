package render

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KIND_MERMAID is the AST node kind for a ```mermaid fenced block.
var KIND_MERMAID = ast.NewNodeKind("Mermaid")

// MERMAID_SEEN_KEY flags a document that contains at least one diagram, so the
// page template only pulls in the mermaid bundle when it is needed.
var MERMAID_SEEN_KEY = parser.NewContextKey()

// MERMAID_LANG is the fence info string that marks a diagram.
const MERMAID_LANG = "mermaid"

// Mermaid holds the raw diagram source; it is handed to the browser verbatim.
type Mermaid struct {
	ast.BaseBlock
	Code []byte
}

func (n *Mermaid) Kind() ast.NodeKind { return KIND_MERMAID }

func (n *Mermaid) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// mermaidTransformer swaps mermaid fences out before the syntax highlighter
// can claim them.
type mermaidTransformer struct{}

func (mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	var fences []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if fcb, ok := n.(*ast.FencedCodeBlock); ok && string(fcb.Language(source)) == MERMAID_LANG {
				fences = append(fences, fcb)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, fcb := range fences {
		parent := fcb.Parent()
		if parent == nil {
			continue
		}
		var code bytes.Buffer
		for i := 0; i < fcb.Lines().Len(); i++ {
			line := fcb.Lines().At(i)
			code.Write(line.Value(source))
		}
		parent.ReplaceChild(parent, fcb, &Mermaid{Code: code.Bytes()})
		pc.Set(MERMAID_SEEN_KEY, true)
	}
}

func renderMermaid(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	diagram := node.(*Mermaid)
	_, _ = w.WriteString(`<pre class="mermaid">`)
	template.HTMLEscape(w, diagram.Code)
	_, _ = w.WriteString("</pre>\n")
	return ast.WalkSkipChildren, nil
}

func hasMermaid(pc parser.Context) bool {
	seen, _ := pc.Get(MERMAID_SEEN_KEY).(bool)
	return seen
}
