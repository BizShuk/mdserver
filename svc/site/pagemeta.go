package site

import (
	"bufio"
	"os"
	"strings"
)

const (
	// FRONTMATTER_FENCE opens and closes a YAML frontmatter block.
	FRONTMATTER_FENCE = "---"
	// MAX_META_LINES bounds how far into a file the metadata scan reads.
	// Listings call this once per page, so it must stay cheap.
	MAX_META_LINES = 200
)

// PageMeta is the subset of a page's frontmatter that listings display.
type PageMeta struct {
	Title       string
	Description string
	Date        string
}

// ReadPageMeta pulls a page's title, description and date without rendering
// it. Frontmatter wins; a missing title falls back to the first ATX heading.
// Unreadable files yield a zero PageMeta rather than an error, since a listing
// is more useful with a blank row than with none.
func ReadPageMeta(fsPath string) PageMeta {
	file, err := os.Open(fsPath)
	if err != nil {
		return PageMeta{}
	}
	defer file.Close()

	var (
		meta          PageMeta
		scanner       = bufio.NewScanner(file)
		inFrontmatter bool
		firstLine     = true
	)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for lines := 0; lines < MAX_META_LINES && scanner.Scan(); lines++ {
		line := strings.TrimRight(scanner.Text(), " \t\r")

		if firstLine {
			firstLine = false
			if strings.TrimSpace(line) == FRONTMATTER_FENCE {
				inFrontmatter = true
				continue
			}
		}

		if inFrontmatter {
			if strings.TrimSpace(line) == FRONTMATTER_FENCE {
				inFrontmatter = false
				if meta.Title != "" {
					return meta
				}
				continue
			}
			applyFrontmatterLine(&meta, line)
			continue
		}

		if meta.Title == "" && strings.HasPrefix(line, "# ") {
			meta.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			return meta
		}
	}
	return meta
}

// applyFrontmatterLine reads a top-level `key: value` pair. Indented lines are
// nested values (list items, mappings) and are skipped, so a `tags:` list can
// never be mistaken for a title.
func applyFrontmatterLine(meta *PageMeta, line string) {
	if line == "" || line != strings.TrimLeft(line, " \t") {
		return
	}
	key, value, found := strings.Cut(line, ":")
	if !found {
		return
	}
	value = unquote(strings.TrimSpace(value))
	if value == "" {
		return
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "title":
		meta.Title = value
	case "description":
		meta.Description = value
	case "date":
		meta.Date = value
	}
}

// unquote strips a single matched pair of surrounding quotes.
func unquote(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if first == last && (first == '"' || first == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
