package moduledoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestModuleValidationOriginal tests the strict validation logic of the original implementation
func TestModuleValidationOriginal(t *testing.T) {
	t.Run("ValidModulePattern", func(t *testing.T) {
		// Test the valid pattern: registration + implementation + static ID
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(ValidModule))
}

type ValidModule struct{}

func (*ValidModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.test.valid",
		New: func() caddy.Module { return new(ValidModule) },
	}
}
`

		// This should work with the original implementation
		testModuleSource(t, source, true, "Valid module should be detected")
	})

	t.Run("MissingImplementation", func(t *testing.T) {
		// Test registration without implementation
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(IncompleteModule))
}

type IncompleteModule struct{}
// Missing CaddyModule() method
`

		// This should fail in original implementation
		testModuleSource(t, source, false, "Module without implementation should fail")
	})

	t.Run("ConstantModuleID", func(t *testing.T) {
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(ComputedModule))
}

type ComputedModule struct{}

func (*ComputedModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: caddy.ModuleID("computed." + "dynamic"),
		New: func() caddy.Module { return new(ComputedModule) },
	}
}
`

		testModuleSource(t, source, true, "Constant module ID should be detected")
	})

	t.Run("NonStaticModuleID", func(t *testing.T) {
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(ComputedModule))
}

type ComputedModule struct{}

var moduleID = "computed.dynamic"

func (*ComputedModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: caddy.ModuleID(moduleID),
		New: func() caddy.Module { return new(ComputedModule) },
	}
}
`

		testModuleSource(t, source, false, "Runtime-computed module ID should be skipped")
	})

	t.Run("constModuleID", func(t *testing.T) {
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(ConstIDModule))
}

type ConstIDModule struct{}

const moduleID = "const.mod"

func (*ConstIDModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: caddy.ModuleID(moduleID),
		New: func() caddy.Module { return new(ConstIDModule) },
	}
}
`

		testModuleSource(t, source, true, "const module ID should be successful")
	})
}

// testModuleSource is a helper that tests module detection on source code
func testModuleSource(t *testing.T, source string, expectSuccess bool, message string) {
	t.Helper()
	fset := token.NewFileSet()
	// Accept any registration argument so invalid module fixtures reach validation.
	caddyFile, err := parser.ParseFile(fset, "caddy.go", `package caddy
type ModuleID string
type Module interface { CaddyModule() ModuleInfo }
type ModuleInfo struct {
	ID ModuleID
	New func() Module
}
func RegisterModule(any) {}
`, 0)
	if err != nil {
		t.Fatalf("Failed to parse Caddy stub: %v", err)
	}
	caddyPkg, err := new(types.Config).Check(caddyCorePackagePath, fset, []*ast.File{caddyFile}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check Caddy stub: %v", err)
	}

	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
		Defs:  make(map[*ast.Ident]types.Object),
	}
	cfg := types.Config{Importer: registrationTestImporter{caddyPkg}}
	typesPkg, err := cfg.Check("test", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("Failed to type-check source: %v", err)
	}

	pkg := &packages.Package{
		Name:      "test",
		Syntax:    []*ast.File{file},
		Types:     typesPkg,
		TypesInfo: info,
	}

	driver := New(nil)

	moduleIdents, err := driver.findCaddyModuleIdents(pkg)

	if expectSuccess {
		if err != nil {
			t.Errorf("%s: Expected success but got error: %v", message, err)
		}
		if len(moduleIdents) == 0 {
			t.Errorf("%s: Expected to find modules but found none", message)
		}
		t.Logf("%s: ✓ Found %d modules", message, len(moduleIdents))
	} else {
		if err == nil && len(moduleIdents) > 0 {
			t.Errorf("%s: Expected failure/skip but found modules: %v", message, moduleIdents)
		}
		t.Logf("%s: ✓ Correctly failed or skipped as expected", message)
	}
}
