package render

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KIND_ALERT is the AST node kind for a GitHub alert (`> [!NOTE]`).
var KIND_ALERT = ast.NewNodeKind("Alert")

// ALERT_MARKER matches the first line of a GitHub alert blockquote.
var ALERT_MARKER = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)

// ALERT_ICONS holds the octicon path data GitHub uses for each alert type.
var ALERT_ICONS = map[string]string{
	"note":      `<path d="M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path>`,
	"tip":       `<path d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z"></path>`,
	"important": `<path d="M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path>`,
	"warning":   `<path d="M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"></path>`,
	"caution":   `<path d="M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"></path>`,
}

// Alert is a GitHub-style callout block.
type Alert struct {
	ast.BaseBlock
	AlertType string
}

func (n *Alert) Kind() ast.NodeKind { return KIND_ALERT }

func (n *Alert) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"AlertType": n.AlertType}, nil)
}

// alertTransformer rewrites blockquotes whose first line is an alert marker
// into Alert nodes.
type alertTransformer struct{}

func (alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if bq, ok := n.(*ast.Blockquote); ok {
				quotes = append(quotes, bq)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, bq := range quotes {
		alertType, ok := stripAlertMarker(bq, source)
		if !ok {
			continue
		}
		parent := bq.Parent()
		if parent == nil {
			continue
		}
		alert := &Alert{AlertType: alertType}
		for child := bq.FirstChild(); child != nil; {
			next := child.NextSibling()
			bq.RemoveChild(bq, child)
			alert.AppendChild(alert, child)
			child = next
		}
		parent.ReplaceChild(parent, bq, alert)
	}
}

// stripAlertMarker detects the `[!TYPE]` first line and removes it from the
// blockquote, reporting the lowercased alert type.
func stripAlertMarker(bq *ast.Blockquote, source []byte) (string, bool) {
	para, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok || para.Lines().Len() == 0 {
		return "", false
	}

	first := para.Lines().At(0)
	match := ALERT_MARKER.FindSubmatch(bytes.TrimSpace(first.Value(source)))
	if match == nil {
		return "", false
	}

	// AST transformers run after inline parsing, so the marker exists twice: as
	// the source line above and as the inline nodes goldmark already built from
	// it. Rewriting the lines alone leaves `[!NOTE]` visible in the output.
	removeInlinePrefix(para, first.Stop)

	if para.Lines().Len() == 1 {
		bq.RemoveChild(bq, para)
	} else {
		rest := text.NewSegments()
		for i := 1; i < para.Lines().Len(); i++ {
			rest.Append(para.Lines().At(i))
		}
		para.SetLines(rest)
	}
	return strings.ToLower(string(match[1])), true
}

// removeInlinePrefix drops the inline children parsed from the marker line —
// everything that starts before stop, the marker line's end offset.
func removeInlinePrefix(para *ast.Paragraph, stop int) {
	for child := para.FirstChild(); child != nil; {
		textNode, ok := child.(*ast.Text)
		if !ok || textNode.Segment.Start >= stop {
			return
		}
		next := child.NextSibling()
		para.RemoveChild(para, child)
		child = next
	}
}

func renderAlert(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	alert := node.(*Alert)
	if entering {
		_, _ = w.WriteString(`<div class="md-alert md-alert-` + alert.AlertType + `">`)
		_, _ = w.WriteString(`<p class="md-alert-title">`)
		_, _ = w.WriteString(`<svg class="md-alert-icon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">`)
		_, _ = w.WriteString(ALERT_ICONS[alert.AlertType])
		_, _ = w.WriteString(`</svg>`)
		_, _ = w.WriteString(alertLabel(alert.AlertType))
		_, _ = w.WriteString(`</p>`)
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("</div>\n")
	return ast.WalkContinue, nil
}

func alertLabel(alertType string) string {
	if alertType == "" {
		return ""
	}
	return strings.ToUpper(alertType[:1]) + alertType[1:]
}

// ensure the renderer signature stays compatible with goldmark.
var _ renderer.NodeRendererFunc = renderAlert
