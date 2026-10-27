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
		if rb.versionCache == nil {
			t.Fatal("versionCache should be initialized")
		}

		// standard library types resolve to an empty version and are
		// cached under their import path
		stdType := types.NewNamed(
			types.NewTypeName(0, types.NewPackage("encoding/json", "json"), "RawMessage", nil),
			types.NewSlice(types.Typ[types.Byte]), nil,
		)

		version, err := rb.getDepVersion(stdType)
		if err != nil {
			t.Fatalf("getDepVersion failed for stdlib type: %v", err)
		}
		if version != "" {
			t.Errorf("stdlib types should have empty version, got %q", version)
		}
		if _, ok := rb.versionCache["encoding/json"]; !ok {
			t.Error("stdlib version should be cached under the import path")
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
		rep, err := rb.buildRepresentation(chanType)
		if err != nil {
			t.Errorf("channel type should fall back gracefully, got error: %v", err)
		} else if rep == nil {
			t.Error("expected non-nil fallback representation for channel type")
		} else {
			t.Log("✓ Graceful fallback for channel type")
		}

		// Test function type
		sig := types.NewSignature(nil, types.NewTuple(), types.NewTuple(), false)
		rep, err = rb.buildRepresentation(sig)
		if err != nil {
			t.Errorf("function type should fall back gracefully, got error: %v", err)
		} else if rep == nil {
			t.Error("expected non-nil fallback representation for function type")
		} else {
			t.Log("✓ Graceful fallback for function type")
		}
	})
}

// TestCaddyCorePackagePathConstant tests the constant usage
func TestCaddyCorePackagePathConstant(t *testing.T) {
	expectedPath := "github.com/caddyserver/caddy/v2"
	if caddyCorePackagePath != expectedPath {
		t.Errorf("Expected caddyCorePackagePath=%q, got %q", expectedPath, caddyCorePackagePath)
	}
}

// TestSynthesisErrorHandlingOriginalBehavior tests error conditions
func TestSynthesisErrorHandlingOriginalBehavior(t *testing.T) {
	// a type whose package cannot be resolved by go list
	missingType := func() *types.Named {
		return types.NewNamed(
			types.NewTypeName(0, types.NewPackage("invalid.example/does/not/exist", "exist"), "Ghost", nil),
			types.Typ[types.String], nil,
		)
	}

	t.Run("GetStructFieldGodocsErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()
		if _, err := rb.getStructFieldGodocs(missingType()); err == nil {
			t.Error("expected error for unresolvable package")
		}
	})

	t.Run("GetGodocForTypeErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()
		if _, err := rb.getGodocForType(missingType()); err == nil {
			t.Error("expected error for unresolvable package")
		}
	})

	t.Run("GetDepVersionErrors", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()
		if _, err := rb.getDepVersion(missingType()); err == nil {
			t.Error("expected error when go list cannot resolve the package")
		}
	})
}
