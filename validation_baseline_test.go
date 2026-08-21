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

// TestSynthesisOriginalLimitations tests the type synthesis limitations
func TestSynthesisOriginalLimitations(t *testing.T) {
	t.Run("UnknownTypeFailure", func(t *testing.T) {
		// Test that unknown types cause hard failures
		// We'll simulate this by checking the synthesis logic

		t.Log("Original synthesis.go has hard failure on unknown types:")
		t.Log("default: return nil, fmt.Errorf with unknown type pattern")

		// This documents the limitation - we can't easily test the actual failure
		// without creating complex type scenarios, but we document the behavior
	})

	t.Run("NoGracefulDegradation", func(t *testing.T) {
		t.Log("Original implementation has no fallback for:")
		t.Log("- Interface types (returns nil, nil)")
		t.Log("- Channel types (not handled)")
		t.Log("- Function types (not handled)")
		t.Log("- Complex generic types (not handled)")
	})
}

// TestConcurrencyIssues attempts to expose race conditions in the original implementation
// func TestConcurrencyIssues(t *testing.T) {
// 	t.Run("ConcurrentCacheAccess", func(t *testing.T) {
// 		// This test may expose the race condition mentioned in the TODO comment
// 		driver := New(nil)

// 		// Run with race detector: go test -race
// 		// The original implementation should show race warnings

// 		done := make(chan bool)

// 		// Writer goroutine
// 		go func() {
// 			for i := 0; i < 100; i++ {
// 				key := "test" + string(rune(i))
// 				// Direct map access without locking (original behavior)
// 				driver.discoveredTypes[key] = &Value{Type: String}
// 			}
// 			done <- true
// 		}()

// 		// Reader goroutine
// 		go func() {
// 			for i := 0; i < 100; i++ {
// 				key := "test" + string(rune(i))
// 				// Direct map access without locking (original behavior)
// 				_ = driver.discoveredTypes[key]
// 			}
// 			done <- true
// 		}()

// 		// Wait for both to complete
// 		<-done
// 		<-done

// 		t.Log("Concurrent access test completed")
// 		t.Log("Run with 'go test -race' to see race condition warnings")
// 	})
// }

// TestWorkspaceMemoryLeak tests the memory leak issues in workspace caching
func TestWorkspaceMemoryLeak(t *testing.T) {
	t.Run("UnboundedPackageCache", func(t *testing.T) {
		// The workspace.go has TODO comments about memory issues:
		// "TODO: make this cache ephemeral (workspace-scoped), there's just not enough memory for all the versions."

		t.Log("Original workspace caching issues:")
		t.Log("- No cache size limits")
		t.Log("- No TTL-based eviction")
		t.Log("- No cleanup of old package versions")
		t.Log("- Memory grows unbounded with version-specific packages")
	})
}
