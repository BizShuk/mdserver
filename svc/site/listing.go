package site

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MAX_COUNT_WALK caps the recursive page count for a directory so a listing
// can never be turned into an unbounded filesystem walk.
const MAX_COUNT_WALK = 20000

// Entry is one row of a directory listing.
type Entry struct {
	// Name is the on-disk name, without the .md extension for pages.
	Name string
	// URL is relative to the directory being listed, so it stays correct
	// regardless of where the directory is mounted.
	URL   string
	IsDir bool
	// Title and Date come from the page's frontmatter or first heading.
	Title string
	Date  string
	// PageCount is the number of markdown files under a directory, recursive.
	PageCount int
	ModTime   time.Time
}

// Listing enumerates a directory's subdirectories and markdown pages. The
// directory's own index file is left out, since the caller renders it above
// the listing. Results are cached briefly; see LISTING_CACHE_TTL.
func (s *Site) Listing(target Target) ([]Entry, error) {
	if target.Kind != KIND_DIR {
		return nil, nil
	}
	return s.listings.get(target.FSPath, func() ([]Entry, error) {
		return s.buildListing(target)
	})
}

func (s *Site) buildListing(target Target) ([]Entry, error) {
	dirEntries, err := os.ReadDir(target.FSPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", target.FSPath, err)
	}

	indexName := ""
	if target.IndexPath != "" {
		indexName = filepath.Base(target.IndexPath)
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if strings.HasPrefix(name, ".") || name == indexName {
			continue
		}

		fsPath := filepath.Join(target.FSPath, name)
		info, err := dirEntry.Info()
		if err != nil {
			continue
		}
		// Follow symlinks far enough to know whether this is a directory.
		if info.Mode()&os.ModeSymlink != 0 {
			if resolved, err := os.Stat(fsPath); err == nil {
				info = resolved
			} else {
				continue
			}
		}

		switch {
		case info.IsDir():
			count, err := countPages(fsPath)
			if err != nil {
				continue
			}
			if count == 0 {
				continue
			}
			entries = append(entries, Entry{
				Name:      name,
				URL:       escapeSegment(name) + "/",
				IsDir:     true,
				PageCount: count,
				ModTime:   info.ModTime(),
			})
		case strings.EqualFold(filepath.Ext(name), MD_EXTENSION):
			base := strings.TrimSuffix(name, filepath.Ext(name))
			meta := ReadPageMeta(fsPath)
			entries = append(entries, Entry{
				Name:    base,
				URL:     escapeSegment(base),
				Title:   meta.Title,
				Date:    meta.Date,
				ModTime: info.ModTime(),
			})
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// countPages counts markdown files under dir, recursively, ignoring hidden
// directories.
func countPages(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable subtree just contributes nothing
		}
		name := d.Name()
		if d.IsDir() {
			if walkPath != dir && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), MD_EXTENSION) && !strings.HasPrefix(name, ".") {
			count++
			if count >= MAX_COUNT_WALK {
				return fs.SkipAll
			}
		}
		return nil
	})
	return count, err
}

// escapeSegment percent-encodes a single URL path segment, so filenames
// containing spaces, "#" or "?" still link correctly.
func escapeSegment(name string) string {
	return url.PathEscape(name)
}
