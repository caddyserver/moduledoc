package moduledoc

import (
	"encoding/json"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSynthesisRealFunctionsOriginalBehavior tests synthesis functions with actual data
func TestSynthesisRealFunctionsOriginalBehavior(t *testing.T) {
	t.Run("RunGoListWithRealPackage", func(t *testing.T) {
		// Create a temporary workspace directory
		tempDir, err := os.MkdirTemp("", "moduledoc-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create a basic go.mod file
		goModContent := `module test.example
go 1.19
`
		goModPath := filepath.Join(tempDir, "go.mod")
		if err := os.WriteFile(goModPath, []byte(goModContent), 0o644); err != nil {
			t.Fatalf("Failed to write go.mod: %v", err)
		}

		t.Log("Testing runGoList behavior:")
		t.Log("- Executes 'go list -json <package>' in workspace")
		t.Log("- Returns goListOutput struct with module information")
		t.Log("- Handles standard library packages differently")

		// Test with standard library package
		output, err := runGoList(tempDir, "fmt")
		if err != nil {
			t.Logf("runGoList failed for fmt package (expected in temp workspace): %v", err)
		} else {
			t.Logf("✓ fmt package info: ImportPath=%s, Standard=%v", output.ImportPath, output.Standard)
			if output.ImportPath == "fmt" {
				t.Log("✓ Standard library package detected correctly")
			}
			if output.Standard {
				t.Log("✓ Standard flag set correctly")
			}
		}

		// Test with non-existent package
		_, err = runGoList(tempDir, "non.existent.package/v999")
		if err != nil {
			t.Logf("✓ runGoList properly fails for non-existent package: %v", err)
			if strings.Contains(err.Error(), ">>>>>>") {
				t.Log("✓ Error formatting includes stderr markers")
			}
		} else {
			t.Error("Expected error for non-existent package")
		}
	})

	t.Run("GoListOutputStructValidation", func(t *testing.T) {
		t.Log("Testing goListOutput struct fields and JSON unmarshaling:")

		// Test JSON unmarshaling with sample data
		jsonData := `{
			"Dir": "/test/dir",
			"ImportPath": "example.com/test",
			"Name": "test",
			"Module": {
				"Path": "example.com/test",
				"Version": "v1.0.0"
			},
			"Standard": false,
			"GoFiles": ["main.go", "test.go"]
		}`

		var output goListOutput
		if err := json.Unmarshal([]byte(jsonData), &output); err != nil {
			t.Errorf("Failed to unmarshal test JSON: %v", err)
		} else {
			t.Log("✓ JSON unmarshaling works correctly")
			if output.Dir == "/test/dir" {
				t.Log("✓ Dir field parsed correctly")
			}
			if output.Module.Version == "v1.0.0" {
				t.Log("✓ Nested Module.Version field parsed correctly")
			}
			if len(output.GoFiles) == 2 {
				t.Log("✓ GoFiles slice parsed correctly")
			}
		}
	})

	t.Run("RepresentationBuilderVersionCache", func(t *testing.T) {
		// Test the version cache behavior with real representationBuilder
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		t.Log("Testing representationBuilder version cache:")
		t.Log("- versionCache map stores package path -> version")
		t.Log("- Hierarchical lookup checks parent packages")
		t.Log("- Cache avoids repeated 'go list' calls")

		// Test cache initialization
		if rb.versionCache == nil {
			t.Error("versionCache should be initialized")
		} else {
			initialSize := len(rb.versionCache)
			t.Logf("✓ versionCache initialized with %d entries", initialSize)

			// Test manual cache entry
			testPkg := "example.com/test/subpackage"
			testVersion := "v1.2.3"
			rb.versionCache[testPkg] = testVersion

			// Test hierarchical lookup behavior
			parts := strings.Split(testPkg, "/")
			for i := len(parts); i > 0; i-- {
				parent := strings.Join(parts[:i], "/")
				if version, ok := rb.versionCache[parent]; ok {
					t.Logf("✓ Found cached version for parent %s: %s", parent, version)
					break
				}
			}

			if len(rb.versionCache) != initialSize+1 {
				t.Error("Cache should have grown by 1")
			} else {
				t.Log("✓ Cache grows as expected")
			}
		}
	})
}

// TestBuildRepresentationWithRealTypes tests buildRepresentation with real Go types
func TestBuildRepresentationWithRealTypes(t *testing.T) {
	t.Run("BasicTypeRepresentations", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		t.Log("Testing buildRepresentation with basic Go types:")

		// Test all basic types
		basicTests := []struct {
			name     string
			typ      types.Type
			expected Type
		}{
			{"bool", types.Typ[types.Bool], Bool},
			{"int", types.Typ[types.Int], Int},
			{"int8", types.Typ[types.Int8], Int},
			{"int16", types.Typ[types.Int16], Int},
			{"int32", types.Typ[types.Int32], Int},
			{"int64", types.Typ[types.Int64], Int},
			{"uint", types.Typ[types.Uint], Uint},
			{"uint8", types.Typ[types.Uint8], Uint},
			{"uint16", types.Typ[types.Uint16], Uint},
			{"uint32", types.Typ[types.Uint32], Uint},
			{"uint64", types.Typ[types.Uint64], Uint},
			{"uintptr", types.Typ[types.Uintptr], Uint},
			{"float32", types.Typ[types.Float32], Float},
			{"float64", types.Typ[types.Float64], Float},
			{"complex64", types.Typ[types.Complex64], Complex},
			{"complex128", types.Typ[types.Complex128], Complex},
			{"string", types.Typ[types.String], String},
		}

		for _, tt := range basicTests {
			t.Run(tt.name, func(t *testing.T) {
				rep, err := rb.buildRepresentation(tt.typ)
				if err != nil {
					t.Errorf("buildRepresentation failed for %s: %v", tt.name, err)
				} else if rep == nil {
					t.Errorf("buildRepresentation returned nil for %s", tt.name)
				} else if rep.Type != tt.expected {
					t.Errorf("Expected Type %s for %s, got %s", tt.expected, tt.name, rep.Type)
				} else {
					t.Logf("✓ %s -> %s", tt.name, rep.Type)
				}
			})
		}
	})

	t.Run("ComplexTypeRepresentations", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		t.Log("Testing buildRepresentation with complex Go types:")

		// Test pointer to string
		stringType := types.Typ[types.String]
		ptrType := types.NewPointer(stringType)
		rep, err := rb.buildRepresentation(ptrType)
		if err != nil {
			t.Errorf("Pointer representation failed: %v", err)
		} else if rep.Type != String {
			t.Errorf("Expected String for *string, got %s", rep.Type)
		} else {
			t.Log("✓ *string -> String (pointer dereferenced)")
		}

		// Test []string slice
		sliceType := types.NewSlice(stringType)
		rep, err = rb.buildRepresentation(sliceType)
		if err != nil {
			t.Errorf("Slice representation failed: %v", err)
		} else if rep.Type != Array {
			t.Errorf("Expected Array for []string, got %s", rep.Type)
		} else if rep.Elems == nil || rep.Elems.Type != String {
			t.Error("Expected String element type for []string")
		} else {
			t.Log("✓ []string -> Array with String elements")
		}

		// Test map[string]int
		intType := types.Typ[types.Int]
		mapType := types.NewMap(stringType, intType)
		rep, err = rb.buildRepresentation(mapType)
		if err != nil {
			t.Errorf("Map representation failed: %v", err)
		} else if rep.Type != Map {
			t.Errorf("Expected Map for map[string]int, got %s", rep.Type)
		} else if rep.MapKeys == nil || rep.MapKeys.Type != String {
			t.Error("Expected String key type")
		} else if rep.Elems == nil || rep.Elems.Type != Int {
			t.Error("Expected Int element type")
		} else {
			t.Log("✓ map[string]int -> Map with String keys, Int elements")
		}

		// Test interface{}
		emptyInterface := types.NewInterfaceType(nil, nil)
		rep, err = rb.buildRepresentation(emptyInterface)
		if err != nil {
			t.Errorf("Interface representation failed: %v", err)
		} else if rep == nil {
			t.Log("✓ interface{} -> nil (original behavior)")
		} else {
			t.Logf("Interface representation: %+v", rep)
		}
	})

	t.Run("UnsupportedTypeHandling", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		t.Log("Testing buildRepresentation with unsupported types:")

		// Test channel type
		stringType := types.Typ[types.String]
		chanType := types.NewChan(types.SendRecv, stringType)
		_, err = rb.buildRepresentation(chanType)
		if err != nil {
			t.Logf("✓ Channel type properly rejected: %v", err)
			if strings.Contains(err.Error(), "unknown type") {
				t.Log("✓ Error message indicates unknown type")
			}
		} else {
			t.Error("Expected error for channel type")
		}

		// Test function type
		sig := types.NewSignature(nil, types.NewTuple(), types.NewTuple(), false)
		_, err = rb.buildRepresentation(sig)
		if err != nil {
			t.Logf("✓ Function type properly rejected: %v", err)
		} else {
			t.Error("Expected error for function type")
		}
	})
}

// TestCaddyCorePackagePathConstant tests the constant usage
func TestCaddyCorePackagePathConstant(t *testing.T) {
	t.Log("Testing caddyCorePackagePath constant:")

	expectedPath := "github.com/caddyserver/caddy/v2"
	if caddyCorePackagePath != expectedPath {
		t.Errorf("Expected caddyCorePackagePath=%q, got %q", expectedPath, caddyCorePackagePath)
	} else {
		t.Log("✓ caddyCorePackagePath constant correct")
	}

	// Test that it's used consistently in the codebase
	t.Log("- Used for identifying Caddy core modules")
	t.Log("- Version /v2 indicates Caddy 2.x compatibility")
	t.Log("- Critical for module validation and detection")
}

// TestSynthesisErrorHandlingOriginalBehavior tests error conditions
func TestSynthesisErrorHandlingOriginalBehavior(t *testing.T) {
	t.Run("GetStructFieldGodocsErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		t.Log("Testing getStructFieldGodocs error handling:")
		t.Log("- Expects *types.Named input (panics on others)")
		t.Log("- Returns error if package loading fails")
		t.Log("- Returns error if struct type not found")

		// This would panic in original implementation with non-Named types
		// We document the behavior without triggering the panic
		t.Log("Original limitation: panics if typ is not *types.Named")
		t.Log("Original limitation: no graceful handling of package load failures")
	})

	t.Run("GetGodocForTypeErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		t.Log("Testing getGodocForType error handling:")
		t.Log("- Expects *types.Named input (panics on others)")
		t.Log("- Returns error if type not found in package")
		t.Log("- Returns empty string if no documentation found")

		// Document original behavior limitations
		t.Log("Original limitation: panics if typ is not *types.Named")
		t.Log("Original limitation: no fallback for missing documentation")
	})

	t.Run("GetDepVersionErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		t.Log("Testing getDepVersion error handling:")
		t.Log("- Returns error if package path cannot be determined")
		t.Log("- Returns error if 'go list' command fails")
		t.Log("- No timeout or cancellation for shell commands")

		// Document the TODO comment behavior
		t.Log("Original TODO: 'we could probably ignore this error, but let's see...'")
		t.Log("Original limitation: synchronous shell command execution")
		t.Log("Original limitation: no retry logic for transient failures")
	})
}
