package moduledoc

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestSynthesisOriginalBehavior tests the core synthesis.go functionality
func TestSynthesisOriginalBehavior(t *testing.T) {
	t.Run("BuildRepresentationBasicTypes", func(t *testing.T) {
		// Test that basic Go types are handled correctly
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Test basic types that should work
		basicTypes := map[string]types.BasicKind{
			"bool":       types.Bool,
			"int":        types.Int,
			"string":     types.String,
			"float64":    types.Float64,
			"complex128": types.Complex128,
		}

		for typeName, kind := range basicTypes {
			basicType := types.Typ[kind]
			rep, err := rb.buildRepresentation(basicType)
			if err != nil {
				t.Errorf("Failed to build representation for %s: %v", typeName, err)
				continue
			}

			if rep == nil {
				t.Errorf("Got nil representation for %s", typeName)
				continue
			}

			t.Logf("✓ Successfully built representation for %s: %v", typeName, rep.Type)
		}
	})

	t.Run("BuildRepresentationInterfaces", func(t *testing.T) {
		// Test interface handling - original returns empty Value
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create a simple interface type
		interfaceType := types.NewInterfaceType(nil, nil)
		rep, err := rb.buildRepresentation(interfaceType)
		if err != nil {
			t.Errorf("Interface handling should not error, but got: %v", err)
		}

		// Original implementation returns new(Value) for interfaces
		if rep == nil {
			t.Error("Expected non-nil representation for interface")
		} else {
			t.Logf("✓ Interface representation: %+v", rep)
		}
	})

	t.Run("BuildRepresentationUnknownType", func(t *testing.T) {
		// Test the "unknown type" error case that causes hard failures
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create a channel type (not handled by original implementation)
		chanType := types.NewChan(types.SendRecv, types.Typ[types.String])
		rep, err := rb.buildRepresentation(chanType)

		// types with no JSON representation get an empty fallback value
		if err != nil {
			t.Errorf("channel type should fall back gracefully, got error: %v", err)
		} else if rep == nil {
			t.Error("expected non-nil fallback representation for channel type")
		} else {
			t.Logf("✓ Graceful fallback for channel type: %+v", rep)
		}
	})

	t.Run("GetDepVersionCaching", func(t *testing.T) {
		// Test the version caching mechanism in getDepVersion
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Verify the version cache is initialized
		if rb.versionCache == nil {
			t.Error("Version cache not initialized in representationBuilder")
		}

		// hierarchical lookup: a type in a subpackage resolves to its cached
		// parent module version without invoking go list
		rb.versionCache["example.com/mod"] = "v1.2.3"
		pkg := types.NewPackage("example.com/mod/sub/pkg", "pkg")
		named := types.NewNamed(
			types.NewTypeName(0, pkg, "Thing", nil),
			types.Typ[types.String], nil,
		)

		version, err := rb.getDepVersion(named)
		if err != nil {
			t.Fatalf("getDepVersion failed: %v", err)
		}
		if version != "v1.2.3" {
			t.Errorf("expected cached parent version v1.2.3, got %q", version)
		}
	})
}

// TestWorkspaceOriginalBehavior tests workspace.go functionality
func TestWorkspaceOriginalBehavior(t *testing.T) {
	t.Run("WorkspaceCreation", func(t *testing.T) {
		driver := New(nil)

		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		// Test workspace structure
		if ws.dir == "" {
			t.Error("Workspace directory not set")
		}

		if ws.goGets == nil {
			t.Error("goGets map not initialized")
		}

		if ws.packagePatterns == nil {
			t.Error("packagePatterns map not initialized")
		}

		if ws.parsedPackages == nil {
			t.Error("parsedPackages map not initialized")
		}

		t.Logf("✓ Workspace created at: %s", ws.dir)

		// Test cleanup
		err = ws.Close()
		if err != nil {
			t.Errorf("Failed to close workspace: %v", err)
		}
		t.Log("✓ Workspace cleaned up successfully")
	})

	t.Run("PackageCachingBehavior", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		// Test the caching mechanism described in TODOs
		t.Log("Original package caching behavior:")
		t.Log("- parsedPackages map stores *packages.Package by key")
		t.Log("- Keys include both versioned and non-versioned forms")
		t.Log("- TODO comment: 'make this cache ephemeral... not enough memory for all versions'")
		t.Log("- No eviction strategy implemented")
		t.Log("- packages.Visit() caches all imported packages")

		// Demonstrate unbounded cache behavior
		testPkg := &packages.Package{
			ID:   "test/package",
			Name: "testpkg",
		}

		// Simulate what happens in the actual code
		ws.mu.Lock()
		ws.parsedPackages["test/package"] = testPkg
		ws.parsedPackages["test/package@v1.0.0"] = testPkg
		ws.parsedPackages["test/package@v1.1.0"] = testPkg
		ws.mu.Unlock()

		if len(ws.parsedPackages) != 3 {
			t.Errorf("Expected 3 cache entries, got %d", len(ws.parsedPackages))
		}

		t.Logf("✓ Cache grew to %d entries (demonstrates unbounded growth)", len(ws.parsedPackages))
	})

	t.Run("ConcurrentAccessPatterns", func(t *testing.T) {
		// Test the mutex usage in workspace
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		t.Log("Workspace concurrency:")
		t.Log("- Has sync.RWMutex for workspace operations")
		t.Log("- Protects goGets map and package operations")
		t.Log("- Driver.discoveredTypes is protected by its own mutex")

		// The workspace itself is properly protected, but the driver cache is not
		testKey := "concurrent/test"

		// This should be safe (workspace level)
		ws.mu.Lock()
		ws.goGets[testKey] = struct{}{}
		ws.mu.Unlock()

		ws.mu.RLock()
		_, exists := ws.goGets[testKey]
		ws.mu.RUnlock()

		if !exists {
			t.Error("Concurrent access to goGets failed")
		} else {
			t.Log("✓ Workspace-level synchronization works correctly")
		}
	})
}

// TestStorageOriginalBehavior tests storage.go functionality
func TestStorageOriginalBehavior(t *testing.T) {
	t.Run("DereferenceFunction", func(t *testing.T) {
		// Test the dereference functionality
		driver := New(nil)

		// Test with no SameAs (should be no-op)
		val := &Value{
			Type:     String,
			TypeName: "test",
		}

		result, err := driver.dereference(val)
		if err != nil {
			t.Errorf("Dereference of non-SameAs value failed: %v", err)
		}

		if result != val {
			t.Error("Dereference should return same value when SameAs is empty")
		}

		t.Log("✓ Dereference no-op case works correctly")
	})

	t.Run("DeepDereferenceRecursion", func(t *testing.T) {
		// Test the recursive deep dereferencing
		driver := New(nil)

		// Create a nested structure to test recursion
		val := &Value{
			Type: Struct,
			StructFields: []*StructField{
				{
					Key: "field1",
					Value: &Value{
						Type:     String,
						TypeName: "string",
					},
				},
			},
		}

		result, err := driver.deepDereference(val)
		if err != nil {
			t.Errorf("DeepDereference failed: %v", err)
		}

		if result == nil {
			t.Error("DeepDereference returned nil")
		}

		if len(result.StructFields) != 1 {
			t.Errorf("Expected 1 struct field, got %d", len(result.StructFields))
		}

		t.Log("✓ DeepDereference recursion works correctly")
	})

	t.Run("SplitLastDotFunction", func(t *testing.T) {
		// Test the utility function that splits fully qualified type names
		testCases := []struct {
			input         string
			expectedLeft  string
			expectedRight string
		}{
			{"github.com/caddyserver/caddy/v2.Config", "github.com/caddyserver/caddy/v2", "Config"},
			{"http.handlers.file_server", "http.handlers", "file_server"},
			{"http", "", "http"},
			{"", "", ""},
		}

		for _, tc := range testCases {
			left, right := SplitLastDot(tc.input)
			if left != tc.expectedLeft || right != tc.expectedRight {
				t.Errorf("SplitLastDot(%q) = (%q, %q), expected (%q, %q)",
					tc.input, left, right, tc.expectedLeft, tc.expectedRight)
			} else {
				t.Logf("✓ SplitLastDot(%q) = (%q, %q)", tc.input, left, right)
			}
		}
	})
}
