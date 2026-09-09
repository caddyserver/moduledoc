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

	t.Run("MissingRegistration", func(t *testing.T) {
		// Test module with implementation but no registration
		source := `
package test

import "github.com/caddyserver/caddy/v2"

type UnregisteredModule struct{}

func (*UnregisteredModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.test.unregistered",
		New: func() caddy.Module { return new(UnregisteredModule) },
	}
}
`

		// This should fail in original implementation
		testModuleSource(t, source, false, "Module without registration should fail")
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

	t.Run("NonStaticModuleID", func(t *testing.T) {
		// Test computed module ID that should be skipped
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

		// This should be skipped (not fail, but not detected) in original implementation
		testModuleSource(t, source, false, "Computed module ID should be skipped")
	})
}

// testModuleSource is a helper that tests module detection on source code
func testModuleSource(t *testing.T, source string, expectSuccess bool, message string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Create a minimal package for testing
	pkg := &packages.Package{
		Name:   "test",
		Syntax: []*ast.File{file},
		TypesInfo: &types.Info{
			Uses: make(map[*ast.Ident]types.Object),
			Defs: make(map[*ast.Ident]types.Object),
		},
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
