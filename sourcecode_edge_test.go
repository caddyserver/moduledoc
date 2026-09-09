package moduledoc

import (
	"os"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadFixturePackage loads one local testdata package with full type info.
func loadFixturePackage(t *testing.T, pattern string) *packages.Package {
	t.Helper()
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps,
		Env: append(os.Environ(), "CGO_ENABLED=0"),
	}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		t.Fatalf("loading %s: %v", pattern, err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package for %s, got %d", pattern, len(pkgs))
	}
	for _, e := range pkgs[0].Errors {
		t.Fatalf("fixture package %s has errors: %v", pattern, e)
	}
	return pkgs[0]
}

func identsByTypeName(t *testing.T, d *Driver, pkg *packages.Package) map[string]string {
	t.Helper()
	idents, err := d.findCaddyModuleIdents(pkg)
	if err != nil {
		t.Fatalf("finding module idents: %v", err)
	}
	byName := make(map[string]string, len(idents))
	for ident, id := range idents {
		byName[ident.Name] = id
	}
	return byName
}

func TestFindModuleIdentsPointerAndValuePatterns(t *testing.T) {
	d := New(newMemStorage())

	// registration via new() with pointer receiver
	pkg := loadFixturePackage(t, "./testdata")
	got := identsByTypeName(t, d, pkg)
	if got["Gizmo"] != "app.namespace.gizmo" {
		t.Errorf("expected Gizmo module, got %#v", got)
	}

	// registration via composite literal with value receiver
	pkg = loadFixturePackage(t, "./testdata/valuerecv")
	got = identsByTypeName(t, d, pkg)
	if got["Sprocket"] != "app.namespace.sprocket" {
		t.Errorf("expected Sprocket module, got %#v", got)
	}
}

func TestModuleIDFromConstant(t *testing.T) {
	d := New(newMemStorage())
	pkg := loadFixturePackage(t, "./testdata/constid")
	got := identsByTypeName(t, d, pkg)
	if got["ConstWidget"] != "app.namespace.const_widget" {
		t.Errorf("module with ID declared as a package constant should be discovered, got %#v", got)
	}
}

func TestModuleIDFromConcatenation(t *testing.T) {
	d := New(newMemStorage())
	pkg := loadFixturePackage(t, "./testdata/exprid")
	got := identsByTypeName(t, d, pkg)
	if got["ExprWidget"] != "app.namespace.expr_widget" {
		t.Errorf("module with ID built from constant string concatenation should be discovered, got %#v", got)
	}
}

func TestUnregisteredModuleFailsPackage(t *testing.T) {
	d := New(newMemStorage())
	pkg := loadFixturePackage(t, "./testdata/unregistered")
	// one unregistered module type fails the whole package,
	// including its valid sibling module (current strict contract)
	if _, err := d.findCaddyModuleIdents(pkg); err == nil {
		t.Error("expected error for package containing an unregistered module type")
	}
}

func TestRegistrationWithoutLocalImplementation(t *testing.T) {
	d := New(newMemStorage())
	pkg := loadFixturePackage(t, "./testdata/noimpl")
	if _, err := d.findCaddyModuleIdents(pkg); err == nil {
		t.Error("expected error for registration whose CaddyModule method is not declared in the package")
	}
}

func TestFindModuleIdentsUnusualAST(t *testing.T) {
	d := New(newMemStorage())
	pkg := loadFixturePackage(t, "./testdata/crosspkg")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registration of a qualified type from another package must not panic: %v", r)
		}
	}()
	// either outcome is acceptable as long as it does not panic:
	// an error (no local implementation) or an empty result
	idents, err := d.findCaddyModuleIdents(pkg)
	if err == nil && len(idents) > 0 {
		t.Logf("cross-package registration produced idents: %v", idents)
	}
}
