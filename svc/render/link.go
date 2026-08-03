package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// MD_EXTENSION is the only file extension mdserver treats as a page.
const MD_EXTENSION = ".md"

// INDEX_BASENAMES are the filenames that stand in for their own directory.
var INDEX_BASENAMES = []string{"README", "index"}

// linkTransformer rewrites in-repo `foo.md` links to the extensionless URLs
// the server actually routes, so markdown that reads correctly on GitHub also
// navigates correctly here.
type linkTransformer struct{}

func (linkTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		if rewritten, changed := RewriteMarkdownLink(string(link.Destination)); changed {
			link.Destination = []byte(rewritten)
		}
		return ast.WalkContinue, nil
	})
}

// RewriteMarkdownLink maps a relative `*.md` destination onto its page URL.
// Absolute URLs, protocol-relative URLs and non-markdown targets pass through
// untouched.
func RewriteMarkdownLink(dest string) (string, bool) {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "//") || strings.HasPrefix(dest, "#") {
		return dest, false
	}
	if strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "tel:") {
		return dest, false
	}

	path, suffix := dest, ""
	if i := strings.IndexAny(dest, "#?"); i >= 0 {
		path, suffix = dest[:i], dest[i:]
	}
	if !strings.HasSuffix(strings.ToLower(path), MD_EXTENSION) {
		return dest, false
	}

	trimmed := path[:len(path)-len(MD_EXTENSION)]
	// An index file addresses its directory, not a sibling page.
	for _, base := range INDEX_BASENAMES {
		if trimmed == base {
			return "./" + suffix, true
		}
		if strings.HasSuffix(trimmed, "/"+base) {
			return trimmed[:len(trimmed)-len(base)] + suffix, true
		}
	}
	return trimmed + suffix, true
}
