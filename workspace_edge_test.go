package moduledoc

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

func tempDocsysDirs(t *testing.T) map[string]bool {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "caddy_docsys_*"))
	if err != nil {
		t.Fatalf("globbing temp dir: %v", err)
	}
	set := make(map[string]bool, len(matches))
	for _, m := range matches {
		set[m] = true
	}
	return set
}

func TestWorkspaceCloseIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	d := New(newMemStorage())
	ws, err := d.openWorkspace()
	if err != nil {
		t.Fatalf("opening workspace: %v", err)
	}
	if err := ws.Close(); err != nil {
		t.Fatalf("closing workspace: %v", err)
	}
	if _, err := os.Stat(ws.dir); !os.IsNotExist(err) {
		t.Errorf("workspace dir should be removed after Close: %v", err)
	}
	if err := ws.Close(); err != nil {
		t.Errorf("closing an already-closed workspace should not error: %v", err)
	}
}

func TestOpenWorkspaceInitFailureCleansTempDir(t *testing.T) {
	before := tempDocsysDirs(t)

	// an empty PATH makes the 'go mod init' exec fail after TempDir succeeds
	t.Setenv("PATH", t.TempDir())

	d := New(newMemStorage())
	if _, err := d.openWorkspace(); err == nil {
		t.Fatal("expected error when the go binary is unavailable")
	}

	after := tempDocsysDirs(t)
	for dir := range after {
		if !before[dir] {
			t.Errorf("temp workspace dir leaked after failed init: %s", dir)
		}
	}
}

func TestGetPackagesEmptyPattern(t *testing.T) {
	ws := workspace{mu: new(sync.RWMutex)}
	if _, err := ws.getPackages("", ""); err == nil {
		t.Error("expected error for empty package pattern")
	}
}

func TestCachedPackagesPartialMiss(t *testing.T) {
	pkgA := &packages.Package{ID: "example.com/mod/a"}
	ws := workspace{
		mu: new(sync.RWMutex),
		packagePatterns: map[string][]string{
			"example.com/mod/...": {"example.com/mod/a", "example.com/mod/b"},
		},
		parsedPackages: map[string]*packages.Package{
			"example.com/mod/a": pkgA,
		},
	}

	// exact key hit
	if got := ws.cachedPackages("example.com/mod/a"); len(got) != 1 || got[0] != pkgA {
		t.Errorf("expected cached package for exact key, got %#v", got)
	}
	// pattern expansion with one member missing must miss entirely
	if got := ws.cachedPackages("example.com/mod/..."); got != nil {
		t.Errorf("expected nil for partially-cached pattern, got %#v", got)
	}
	// unknown pattern
	if got := ws.cachedPackages("example.com/other/..."); got != nil {
		t.Errorf("expected nil for unknown pattern, got %#v", got)
	}
}

func TestPackageKeyVersionSuffix(t *testing.T) {
	pkg := &packages.Package{ID: "example.com/mod/a"}
	if got := packageKey(pkg); got != "example.com/mod/a" {
		t.Errorf("packageKey without module = %q", got)
	}
	pkg.Module = &packages.Module{Path: "example.com/mod"}
	if got := packageKey(pkg); got != "example.com/mod/a" {
		t.Errorf("packageKey with unversioned module = %q", got)
	}
	pkg.Module.Version = "v1.2.3"
	if got := packageKey(pkg); got != "example.com/mod/a@v1.2.3" {
		t.Errorf("packageKey with versioned module = %q", got)
	}
}

func TestAlreadyGotModuleHierarchy(t *testing.T) {
	ws := workspace{
		goGets: map[string]struct{}{"github.com/foo/bar": {}},
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"github.com/foo/bar", true},
		{"github.com/foo/bar/baz", true},
		{"github.com/foo/bar/baz/deep", true},
		// sibling with a shared string prefix is a different module
		{"github.com/foo/barbaz", false},
		{"github.com/foo", false},
		{"github.com", false},
		{"", false},
	} {
		if got := ws.alreadyGotModule(tc.path); got != tc.want {
			t.Errorf("alreadyGotModule(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}
