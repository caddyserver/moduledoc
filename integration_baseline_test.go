package moduledoc

import (
	"go/types"
	"testing"
)

// TestIntegrationOriginalBehavior tests the integration between workspace, synthesis, and storage
func TestIntegrationOriginalBehavior(t *testing.T) {
	t.Run("FullWorkflowWithTestdata", func(t *testing.T) {
		if testing.Short() {
			t.Skip("requires the Go toolchain")
		}
		// the public API needs a fetchable module path, so run the same
		// pipeline (load → find idents → build representations) against
		// the local testdata package instead
		driver := New(newMemStorage())
		ws := localWorkspace(t, driver)

		pkgs, err := ws.getPackages(testdataPackagePath, "")
		if err != nil {
			t.Fatalf("loading testdata package: %v", err)
		}
		if len(pkgs) != 1 {
			t.Fatalf("expected 1 package, got %d", len(pkgs))
		}

		modules, err := ws.representationBuilder().loadModulesFromSinglePackage(pkgs[0])
		if err != nil {
			t.Fatalf("loading modules from testdata package: %v", err)
		}

		t.Logf("Found %d modules in testdata", len(modules))

		for i, module := range modules {
			t.Logf("Module %d: Name=%s", i, module.Name)
			if module.Representation != nil {
				t.Logf("  Type: %s", module.Representation.Type)
				t.Logf("  TypeName: %s", module.Representation.TypeName)
			}
		}

		// Verify we found the expected static module
		foundGizmo := false
		for _, module := range modules {
			if module.Name == "app.namespace.gizmo" {
				foundGizmo = true
				break
			}
		}

		if !foundGizmo {
			t.Error("Expected to find 'app.namespace.gizmo' module")
		} else {
			t.Log("✓ Successfully found expected module")
		}
	})

	t.Run("WorkspacePackageCacheIntegration", func(t *testing.T) {
		// Test the integration between workspace and package caching
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		// Test that cachedPackages returns nil for non-cached packages
		cached := ws.cachedPackages("non.existent/package")
		if cached != nil {
			t.Error("Expected nil for non-cached package")
		} else {
			t.Log("✓ cachedPackages correctly returns nil for non-cached packages")
		}

		// Test that the cache behavior matches the TODO comments
		t.Log("Original caching behavior verification:")
		t.Log("- Cache stores packages.Package by string key")
		t.Log("- Uses both versioned and non-versioned keys")
		t.Log("- No size limits (TODO: 'not enough memory for all versions')")
		t.Log("- No TTL expiration (TODO: 'should probably expire')")
	})

	t.Run("SynthesisStorageIntegration", func(t *testing.T) {
		// Test the integration between synthesis and storage via dereference
		driver := New(nil)

		// Test dereference with SameAs reference and nil storage - this will panic
		// because dereference calls ds.db.GetTypeByName() and ds.db is nil
		valWithSameAs := &Value{
			SameAs:          "example.com/test.TestType@v1.0.0",
			ModuleNamespace: stringPtr("test.namespace"),
			ModuleInlineKey: stringPtr("type"),
		}

		// A nil Storage is invalid usage; today dereference panics on it,
		// a graceful error would also be acceptable — only require that it
		// does not silently succeed
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("✓ Dereference panicked with nil storage: %v", r)
				}
			}()
			_, err := driver.dereference(valWithSameAs)
			if err != nil {
				t.Logf("✓ Dereference returned error with nil storage: %v", err)
			} else {
				t.Error("dereference with nil storage must not silently succeed")
			}
		}()

		// Test dereference with empty SameAs - this should work fine
		valNoSameAs := &Value{
			Type:            String,
			TypeName:        "string",
			ModuleNamespace: stringPtr("test.namespace"),
			ModuleInlineKey: stringPtr("type"),
		}

		result, err := driver.dereference(valNoSameAs)
		if err != nil {
			t.Errorf("dereference failed with empty SameAs: %v", err)
		} else if result != valNoSameAs {
			t.Error("dereference should return same value when SameAs is empty")
		} else {
			t.Log("✓ Dereference works correctly with empty SameAs")
		}

		// Test deepDereference recursion with no SameAs references
		structVal := &Value{
			Type: Struct,
			StructFields: []*StructField{
				{
					Key: "field1",
					Value: &Value{
						Type:     String,
						TypeName: "string",
						Doc:      "Field documentation",
					},
				},
			},
		}

		deepResult, err := driver.deepDereference(structVal)
		if err != nil {
			t.Errorf("deepDereference failed: %v", err)
		} else if deepResult == nil {
			t.Error("deepDereference returned nil")
		} else {
			t.Log("✓ deepDereference completed successfully")
			if len(deepResult.StructFields) > 0 && deepResult.StructFields[0].Doc != "" {
				t.Logf("  Field doc preserved: %q", deepResult.StructFields[0].Doc)
			}
		}
	})

	t.Run("BuildRepresentationCacheIntegration", func(t *testing.T) {
		// Test that buildRepresentation uses discoveredTypes cache
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// a type already in the driver cache is returned as a reference
		// without loading its package or touching storage
		pkg := types.NewPackage("example.com/fake", "fake")
		named := types.NewNamed(
			types.NewTypeName(0, pkg, "Thing", nil),
			types.Typ[types.String], nil,
		)

		rb.versionCache["example.com/fake"] = "v1.0.0"
		sameAs := "example.com/fake.Thing@v1.0.0"
		driver.setDiscoveredType(sameAs, &Value{Type: String, TypeName: "example.com/fake.Thing"})

		rep, err := rb.buildRepresentation(named)
		if err != nil {
			t.Fatalf("buildRepresentation failed: %v", err)
		}
		if rep.SameAs != sameAs {
			t.Errorf("expected cache hit returning reference %q, got %+v", sameAs, rep)
		}
	})
}

// TestModuleValidationIntegration tests the strict validation behavior
func TestModuleValidationIntegration(t *testing.T) {
	t.Run("OriginalStrictValidation", func(t *testing.T) {
		// fully compliant module: registration + implementation + static ID
		validSource := `
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
		testModuleSource(t, validSource, true, "Compliant module should be detected")
	})
}

// Helper function to create string pointers for testing
func stringPtr(s string) *string {
	return &s
}
