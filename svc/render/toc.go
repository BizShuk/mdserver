package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

const (
	// TOC_MIN_LEVEL and TOC_MAX_LEVEL bound which headings reach the sidebar.
	// H1 is the page title and is rendered separately.
	TOC_MIN_LEVEL = 2
	TOC_MAX_LEVEL = 3
)

// HEADINGS_KEY carries the collected headings out of the parse.
var HEADINGS_KEY = parser.NewContextKey()

// Heading is one entry of a page's table of contents.
type Heading struct {
	Level int
	Text  string
	ID    string
}

// tocTransformer records every heading during the parse, so neither the title
// nor the table of contents costs a second pass over the document.
type tocTransformer struct{}

func (tocTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var headings []Heading

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		id, _ := h.AttributeString("id")
		idText, _ := id.([]byte)
		headings = append(headings, Heading{
			Level: h.Level,
			Text:  strings.TrimSpace(inlineText(h, source)),
			ID:    string(idText),
		})
		return ast.WalkContinue, nil
	})

	pc.Set(HEADINGS_KEY, headings)
}

func collectedHeadings(pc parser.Context) []Heading {
	headings, _ := pc.Get(HEADINGS_KEY).([]Heading)
	return headings
}

// TOC filters the headings down to the levels worth navigating.
func TOC(headings []Heading) []Heading {
	toc := make([]Heading, 0, len(headings))
	for _, h := range headings {
		if h.Level >= TOC_MIN_LEVEL && h.Level <= TOC_MAX_LEVEL && h.ID != "" {
			toc = append(toc, h)
		}
	}
	return toc
}
