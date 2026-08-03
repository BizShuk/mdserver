// Package site maps request paths onto files under the served directory.
//
// The routing rule is the whole configuration: a folder is a route and a
// markdown filename is a page. `/a/b` serves `a/b.md`; `/a/b/` serves the
// directory `a/b`, using its README.md or index.md as the directory's own page.
package site

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// MD_EXTENSION is the extension that is converted to HTML before serving.
const MD_EXTENSION = ".md"

// HTML_EXTENSIONS are served exactly as they are on disk: an HTML file is
// already the output format, so it is never put through the markdown pipeline
// or wrapped in the page template.
var HTML_EXTENSIONS = []string{".html", ".htm"}

// INDEX_FILENAMES stand in for the directory that contains them, in priority
// order.
var INDEX_FILENAMES = []string{"README.md", "index.md", "readme.md", "Index.md"}

// ErrOutsideRoot reports a path that escapes the served directory.
var ErrOutsideRoot = errors.New("path escapes site root")

// Kind classifies what a request path resolved to.
type Kind int

const (
	KIND_NOT_FOUND Kind = iota
	KIND_PAGE           // a markdown file rendered as HTML
	KIND_DIR            // a directory rendered as an index listing
	KIND_STATIC         // any other file, served as-is
	KIND_REDIRECT       // canonical URL differs from the one requested
)

// Target is the resolution of one request path.
type Target struct {
	Kind Kind

	// FSPath is the absolute path on disk. For KIND_DIR it is the directory.
	FSPath string
	// RelPath is FSPath relative to the site root, slash-separated. The root
	// directory itself is "".
	RelPath string
	// URLPath is the canonical URL for this target.
	URLPath string
	// IndexPath is the directory's README/index file, when it has one.
	// Only set for KIND_DIR.
	IndexPath string
	// Redirect is the URL to send the client to. Only set for KIND_REDIRECT.
	Redirect string
}

// Site serves markdown from a single root directory.
type Site struct {
	Root  string
	Title string

	listings *ttlCache[[]Entry]
}

// New roots a Site at dir, which must exist and be a directory.
func New(dir string) (*Site, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve root %q: %w", dir, err)
	}
	// Resolve symlinks so the containment check below compares real paths.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat root %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root %q is not a directory", abs)
	}
	return &Site{
		Root:     abs,
		Title:    filepath.Base(abs),
		listings: newTTLCache[[]Entry](LISTING_CACHE_TTL),
	}, nil
}

// Resolve maps a request path to a Target. urlPath is the already-unescaped
// path from the request.
func (s *Site) Resolve(urlPath string) (Target, error) {
	trailingSlash := strings.HasSuffix(urlPath, "/")
	clean := path.Clean("/" + strings.TrimPrefix(urlPath, "/"))
	rel := strings.TrimPrefix(clean, "/")

	if hasHiddenSegment(rel) {
		return Target{Kind: KIND_NOT_FOUND}, nil
	}

	fsPath, err := s.join(rel)
	if err != nil {
		return Target{}, err
	}

	// The site root is always a directory listing.
	if rel == "" {
		return s.dirTarget(fsPath, "", "/"), nil
	}

	// `/a/b.md` is written in markdown links but `/a/b` is what we route.
	if strings.EqualFold(path.Ext(rel), MD_EXTENSION) {
		return Target{Kind: KIND_REDIRECT, Redirect: canonicalPageURL(clean)}, nil
	}

	// With a trailing slash the client is asking for a directory.
	if trailingSlash {
		if info, err := os.Stat(fsPath); err == nil && info.IsDir() {
			return s.dirTarget(fsPath, rel, clean+"/"), nil
		}
		// Not a directory: drop the slash and try again as a page.
		return Target{Kind: KIND_REDIRECT, Redirect: clean}, nil
	}

	// A page wins over a same-named directory; the directory stays reachable
	// at its trailing-slash URL.
	if pagePath := fsPath + MD_EXTENSION; isFile(pagePath) {
		return Target{
			Kind:    KIND_PAGE,
			FSPath:  pagePath,
			RelPath: rel + MD_EXTENSION,
			URLPath: clean,
		}, nil
	}

	// `/report` also finds report.html, but markdown keeps priority so the
	// extensionless URL of a page never changes meaning when an export lands
	// next to it. The file is still served as-is, not rendered.
	for _, ext := range HTML_EXTENSIONS {
		if htmlPath := fsPath + ext; isFile(htmlPath) {
			return Target{
				Kind:    KIND_STATIC,
				FSPath:  htmlPath,
				RelPath: rel + ext,
				URLPath: clean,
			}, nil
		}
	}

	info, err := os.Stat(fsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Target{Kind: KIND_NOT_FOUND}, nil
		}
		return Target{}, fmt.Errorf("stat %q: %w", fsPath, err)
	}
	if info.IsDir() {
		return Target{Kind: KIND_REDIRECT, Redirect: clean + "/"}, nil
	}
	return Target{
		Kind:    KIND_STATIC,
		FSPath:  fsPath,
		RelPath: rel,
		URLPath: clean,
	}, nil
}

// dirTarget describes a directory, noting its index page when one exists.
func (s *Site) dirTarget(fsPath, rel, urlPath string) Target {
	target := Target{
		Kind:    KIND_DIR,
		FSPath:  fsPath,
		RelPath: rel,
		URLPath: urlPath,
	}
	for _, name := range INDEX_FILENAMES {
		candidate := filepath.Join(fsPath, name)
		if isFile(candidate) {
			target.IndexPath = candidate
			break
		}
	}
	return target
}

// join builds an absolute path for rel and refuses anything outside the root.
func (s *Site) join(rel string) (string, error) {
	joined := filepath.Join(s.Root, filepath.FromSlash(rel))
	if joined != s.Root && !strings.HasPrefix(joined, s.Root+string(filepath.Separator)) {
		return "", ErrOutsideRoot
	}
	return joined, nil
}

// canonicalPageURL strips the .md extension, mapping index files onto their
// own directory.
func canonicalPageURL(clean string) string {
	trimmed := strings.TrimSuffix(clean, path.Ext(clean))
	base := path.Base(trimmed)
	for _, name := range INDEX_FILENAMES {
		if base == strings.TrimSuffix(name, MD_EXTENSION) {
			dir := path.Dir(trimmed)
			if dir == "/" {
				return "/"
			}
			return dir + "/"
		}
	}
	return trimmed
}

func hasHiddenSegment(rel string) bool {
	for _, segment := range strings.Split(rel, "/") {
		if strings.HasPrefix(segment, ".") && segment != "" {
			return true
		}
	}
	return false
}

// isHTMLName reports whether a filename is served as-is rather than rendered.
// It takes a bare name or a full path.
func isHTMLName(name string) bool {
	ext := filepath.Ext(name)
	for _, candidate := range HTML_EXTENSIONS {
		if strings.EqualFold(ext, candidate) {
			return true
		}
	}
	return false
}

func isFile(fsPath string) bool {
	info, err := os.Stat(fsPath)
	return err == nil && info.Mode().IsRegular()
}
