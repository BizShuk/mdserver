package render

import (
	"bytes"
	"fmt"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

const (
	// CHROMA_STYLE_LIGHT and CHROMA_STYLE_DARK are the chroma themes used for
	// the two colour schemes. The light one is also what the highlighter is
	// configured with, since class-based output makes the choice cosmetic.
	CHROMA_STYLE_LIGHT = "github"
	CHROMA_STYLE_DARK  = "github-dark"
)

// SyntaxCSS builds the stylesheet for highlighted code: the light theme at top
// level, the dark theme nested in a prefers-color-scheme query. Generating it
// here keeps the themes and the highlighter from drifting apart.
func SyntaxCSS() ([]byte, error) {
	light, err := styleCSS(CHROMA_STYLE_LIGHT)
	if err != nil {
		return nil, err
	}
	dark, err := styleCSS(CHROMA_STYLE_DARK)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.Write(light)
	out.WriteString("\n@media (prefers-color-scheme: dark) {\n")
	out.Write(dark)
	out.WriteString("\n}\n")
	return out.Bytes(), nil
}

func styleCSS(name string) ([]byte, error) {
	style := styles.Get(name)
	if style == nil {
		return nil, fmt.Errorf("chroma style %q not found", name)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var buf bytes.Buffer
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return nil, fmt.Errorf("write chroma css for %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
