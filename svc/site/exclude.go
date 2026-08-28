package site

import (
	"fmt"
	"path"
	"strings"
)

// exclusions hides part of the served tree. It exists because a site root is
// often a working directory rather than a folder curated for reading: the
// notes are the site, but the tooling that produces them sits in the same
// checkout. Excluding is the only alternative to copying the readable subset
// somewhere else, which would break `編輯即所見`.
//
// Two pattern shapes, distinguished by whether the pattern contains a slash:
//
//	skills      matches that name at any depth (a bare name, or a glob)
//	docs/specs  matches that exact path from the root down (anchored)
//
// A match on any ancestor hides the whole subtree, so excluding a directory
// needs one pattern, not one per file. Globbing is path.Match, whose `*` does
// not cross a separator — `docs/*` is one level, never the subtree.
type exclusions struct {
	// names are matched against a single path segment, at any depth.
	names []string
	// anchored are matched against the whole root-relative path.
	anchored []string
}

// newExclusions compiles patterns, rejecting malformed globs at startup rather
// than silently matching nothing on every request.
func newExclusions(patterns []string) (exclusions, error) {
	var compiled exclusions
	for _, pattern := range patterns {
		pattern = strings.Trim(strings.TrimSpace(pattern), "/")
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, "probe"); err != nil {
			return exclusions{}, fmt.Errorf("bad exclude pattern %q: %w", pattern, err)
		}
		if strings.Contains(pattern, "/") {
			compiled.anchored = append(compiled.anchored, pattern)
			continue
		}
		compiled.names = append(compiled.names, pattern)
	}
	return compiled, nil
}

func (e exclusions) empty() bool {
	return len(e.names) == 0 && len(e.anchored) == 0
}

// match reports whether rel, a slash-separated root-relative path, is hidden.
// Every ancestor is tested, so a hidden directory takes its contents with it.
func (e exclusions) match(rel string) bool {
	if e.empty() || rel == "" {
		return false
	}
	segments := strings.Split(rel, "/")
	for i, segment := range segments {
		for _, pattern := range e.names {
			if ok, _ := path.Match(pattern, segment); ok {
				return true
			}
		}
		prefix := strings.Join(segments[:i+1], "/")
		for _, pattern := range e.anchored {
			if ok, _ := path.Match(pattern, prefix); ok {
				return true
			}
		}
	}
	return false
}
