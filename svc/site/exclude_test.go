package site

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTree writes the fixture every test here shares: readable notes next to
// the tooling a working directory accumulates.
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"README.md",
		"notes/trip.md",
		"notes/draft.tmp",
		"skills/find/SKILL.md",
		"docs/guide.md",
		"docs/specs/internal.md",
	}
	for _, name := range files {
		fsPath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fsPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fsPath, []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestNewRejectsBadPattern(t *testing.T) {
	if _, err := New(buildTree(t), []string{"["}); err == nil {
		t.Fatal("expected a malformed glob to fail at startup")
	}
}

func TestResolveHidesExcludedPaths(t *testing.T) {
	site, err := New(buildTree(t), []string{"skills", "docs/specs", "*.tmp"})
	if err != nil {
		t.Fatal(err)
	}

	hidden := []string{
		"/skills/",             // the excluded directory itself
		"/skills/find/SKILL",   // and everything under it
		"/docs/specs/",         // anchored pattern
		"/docs/specs/internal", //
		"/notes/draft.tmp",     // glob on a static file
	}
	for _, urlPath := range hidden {
		target, err := site.Resolve(urlPath)
		if err != nil {
			t.Fatalf("resolve %s: %v", urlPath, err)
		}
		if target.Kind != KIND_NOT_FOUND {
			t.Errorf("resolve %s: kind %d, want KIND_NOT_FOUND", urlPath, target.Kind)
		}
	}

	visible := map[string]Kind{
		"/":           KIND_DIR,
		"/notes/trip": KIND_PAGE,
		"/docs/guide": KIND_PAGE,
		"/docs/":      KIND_DIR,
	}
	for urlPath, want := range visible {
		target, err := site.Resolve(urlPath)
		if err != nil {
			t.Fatalf("resolve %s: %v", urlPath, err)
		}
		if target.Kind != want {
			t.Errorf("resolve %s: kind %d, want %d", urlPath, target.Kind, want)
		}
	}
}

func TestListingOmitsExcludedEntries(t *testing.T) {
	site, err := New(buildTree(t), []string{"skills", "docs/specs"})
	if err != nil {
		t.Fatal(err)
	}

	root, err := site.Resolve("/")
	if err != nil {
		t.Fatal(err)
	}
	names := listingNames(t, site, root)
	for _, unwanted := range []string{"skills"} {
		if names[unwanted] {
			t.Errorf("root listing still shows %q", unwanted)
		}
	}
	for _, wanted := range []string{"notes", "docs"} {
		if !names[wanted] {
			t.Errorf("root listing lost %q", wanted)
		}
	}

	docs, err := site.Resolve("/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if listingNames(t, site, docs)["specs"] {
		t.Error("docs listing still shows the excluded specs directory")
	}
}

// TestListingDropsDirectoryEmptiedByExclusion pins the rule that a page count
// and a listing agree: a directory whose every page is hidden must not be
// offered as a row that leads nowhere.
func TestListingDropsDirectoryEmptiedByExclusion(t *testing.T) {
	site, err := New(buildTree(t), []string{"SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	root, err := site.Resolve("/")
	if err != nil {
		t.Fatal(err)
	}
	if listingNames(t, site, root)["skills"] {
		t.Error("skills is listed although its only page is excluded")
	}
}

func listingNames(t *testing.T, site *Site, target Target) map[string]bool {
	t.Helper()
	entries, err := site.Listing(target)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[entry.Name] = true
	}
	return names
}
