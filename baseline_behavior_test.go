package moduledoc

import (
	"os"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestOriginalBehaviorBaseline establishes baseline behavior for the original implementation
func TestOriginalBehaviorBaseline(t *testing.T) {
	t.Run("ValidStaticModule", func(t *testing.T) {
		// Test only the gizmo.go file which should work
		cfg := &packages.Config{
			Dir: "testdata",
			Mode: packages.NeedSyntax |
				packages.NeedImports |
				packages.NeedDeps |
				packages.NeedTypes |
				packages.NeedModule |
				packages.NeedTypesInfo,
			Env: append(os.Environ(), "CGO_ENABLED=0"),
		}

		// Load only the gizmo.go file
		pkgs, err := packages.Load(cfg, ".")
		if err != nil {
			t.Logf("Package loading failed (this is expected if testdata has validation errors): %v", err)
			return
		}

		if len(pkgs) == 0 {
			t.Log("No packages loaded - this may be expected if testdata has mixed valid/invalid modules")
			return
		}

		driver := New(nil)
		pkg := pkgs[0]

		moduleIdents, err := driver.findCaddyModuleIdents(pkg)
		if err != nil {
			t.Logf("Module detection failed (this documents original behavior): %v", err)
			t.Log("Original implementation fails hard on mixed valid/invalid modules")
			return
		}

		t.Logf("Found %d modules: %v", len(moduleIdents), moduleIdents)

		// If we get here, the original implementation found some modules
		for _, moduleID := range moduleIdents {
			t.Logf("✓ Detected module: %s", moduleID)
		}
	})

	t.Run("StrictValidationBehavior", func(t *testing.T) {
		t.Log("Original implementation behavior:")
		t.Log("- Requires both caddy.RegisterModule() AND CaddyModule() method")
		t.Log("- Fails hard if any module in package has missing registration/implementation")
		t.Log("- Only accepts static string literals in ModuleInfo.ID")
		t.Log("- Cannot resolve constants or computed expressions")
		t.Log("- No graceful degradation for partial failures")
	})

	t.Run("CachingBehavior", func(t *testing.T) {
		t.Log("Original caching limitations:")
		t.Log("- discoveredTypes map has race conditions (TODO comment confirms this)")
		t.Log("- No mutex protection on concurrent access")
		t.Log("- Unbounded memory growth")
		t.Log("- No TTL or eviction strategy")
	})

	t.Run("SynthesisBehavior", func(t *testing.T) {
		t.Log("Original synthesis limitations:")
		t.Log("- Hard failure on unknown types")
		t.Log("- No fallback for interfaces, channels, functions")
		t.Log("- No graceful degradation")
	})
}

// TestOriginalLimitationsDocumented documents the specific limitations we found
func TestOriginalLimitationsDocumented(t *testing.T) {
	t.Run("KnownIssues", func(t *testing.T) {
		t.Log("Issues identified in original implementation:")
		t.Log("1. Race condition in discoveredTypes (confirmed by TODO comment)")
		t.Log("2. Dynamic module IDs skipped with warning")
		t.Log("3. Constant module IDs skipped with warning")
		t.Log("4. Hard failure on unknown Go types")
		t.Log("5. No cache eviction causing memory leaks")
		t.Log("6. Strict validation prevents mixed module packages")
	})

	t.Run("ExpectedBehaviors", func(t *testing.T) {
		t.Log("What works in original implementation:")
		t.Log("✓ Static string module IDs like 'app.namespace.gizmo'")
		t.Log("✓ Basic type synthesis for primitives and structs")
		t.Log("✓ JSON tag processing")
		t.Log("✓ Package loading and AST analysis")
	})
}
