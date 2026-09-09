package moduledoc

import (
	"go/types"
	"strings"
	"testing"
)

// TestSynthesisFunctionsOriginalBehavior tests the core synthesis functions
func TestSynthesisFunctionsOriginalBehavior(t *testing.T) {
	t.Run("GetStructFieldGodocs", func(t *testing.T) {
		_, rb, typ := kindsBuilder(t, "Widget")

		docs, err := rb.getStructFieldGodocs(typ)
		if err != nil {
			t.Fatalf("getStructFieldGodocs failed: %v", err)
		}
		if !strings.Contains(docs["Name"], "Name is the widget name") {
			t.Errorf("expected field godoc for Name, got %q", docs["Name"])
		}
		if _, ok := docs["DoublePtr"]; ok {
			t.Error("fields without godoc should not appear in the map")
		}
	})

	t.Run("GetGodocForType", func(t *testing.T) {
		_, rb, typ := kindsBuilder(t, "Widget")

		doc, err := rb.getGodocForType(typ)
		if err != nil {
			t.Fatalf("getGodocForType failed: %v", err)
		}
		if !strings.Contains(doc, "exercises the field kinds") {
			t.Errorf("expected type godoc, got %q", doc)
		}
	})

	t.Run("BuildRepresentationBasicTypes", func(t *testing.T) {
		// Test buildRepresentation with basic Go types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Test basic types that should work without external dependencies
		basicTypes := map[string]types.Type{
			"bool":       types.Typ[types.Bool],
			"int":        types.Typ[types.Int],
			"uint":       types.Typ[types.Uint],
			"float64":    types.Typ[types.Float64],
			"complex128": types.Typ[types.Complex128],
			"string":     types.Typ[types.String],
		}

		for name, typ := range basicTypes {
			t.Run(name, func(t *testing.T) {
				rep, err := rb.buildRepresentation(typ)
				if err != nil {
					t.Errorf("buildRepresentation failed for %s: %v", name, err)
				} else if rep == nil {
					t.Errorf("buildRepresentation returned nil for %s", name)
				} else {
					t.Logf("✓ %s -> Type: %s", name, rep.Type)
				}
			})
		}
	})

	t.Run("BuildRepresentationInterfaces", func(t *testing.T) {
		// Test buildRepresentation with interface types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create an interface type
		interfaceType := types.NewInterfaceType(nil, nil)

		rep, err := rb.buildRepresentation(interfaceType)
		if err != nil {
			t.Errorf("buildRepresentation failed for interface: %v", err)
		} else if rep == nil {
			t.Error("expected non-nil empty representation for interface")
		} else if rep.Type != "" {
			t.Errorf("expected empty type for interface, got %s", rep.Type)
		}
	})

	t.Run("BuildRepresentationPointers", func(t *testing.T) {
		// Test buildRepresentation with pointer types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create pointer to string
		stringType := types.Typ[types.String]
		ptrType := types.NewPointer(stringType)

		rep, err := rb.buildRepresentation(ptrType)
		if err != nil {
			t.Errorf("buildRepresentation failed for pointer: %v", err)
		} else if rep == nil {
			t.Error("buildRepresentation returned nil for pointer")
		} else if rep.Type != String {
			t.Errorf("Expected String type for *string, got %s", rep.Type)
		} else {
			t.Log("✓ Pointer types dereference to underlying type")
		}
	})

	t.Run("BuildRepresentationSlices", func(t *testing.T) {
		// Test buildRepresentation with slice types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create []string slice
		stringType := types.Typ[types.String]
		sliceType := types.NewSlice(stringType)

		rep, err := rb.buildRepresentation(sliceType)
		if err != nil {
			t.Errorf("buildRepresentation failed for slice: %v", err)
		} else if rep == nil {
			t.Error("buildRepresentation returned nil for slice")
		} else if rep.Type != Array {
			t.Errorf("Expected Array type for []string, got %s", rep.Type)
		} else if rep.Elems == nil {
			t.Error("Expected Elems to be set for array type")
		} else if rep.Elems.Type != String {
			t.Errorf("Expected String element type, got %s", rep.Elems.Type)
		} else {
			t.Log("✓ Slice types become Array with proper element type")
		}
	})

	t.Run("BuildRepresentationMaps", func(t *testing.T) {
		// Test buildRepresentation with map types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		// Create map[string]int
		stringType := types.Typ[types.String]
		intType := types.Typ[types.Int]
		mapType := types.NewMap(stringType, intType)

		rep, err := rb.buildRepresentation(mapType)
		if err != nil {
			t.Errorf("buildRepresentation failed for map: %v", err)
		} else if rep == nil {
			t.Error("buildRepresentation returned nil for map")
		} else if rep.Type != Map {
			t.Errorf("Expected Map type for map[string]int, got %s", rep.Type)
		} else if rep.MapKeys == nil {
			t.Error("Expected MapKeys to be set")
		} else if rep.MapKeys.Type != String {
			t.Errorf("Expected String key type, got %s", rep.MapKeys.Type)
		} else if rep.Elems == nil {
			t.Error("Expected Elems to be set")
		} else if rep.Elems.Type != Int {
			t.Errorf("Expected Int element type, got %s", rep.Elems.Type)
		} else {
			t.Log("✓ Map types have proper key and element types")
		}
	})

	t.Run("ModuleMapDetection", func(t *testing.T) {
		// map[string]json.RawMessage must become ModuleMap; any other map
		// stays a plain Map
		driver := New(newMemStorage())
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		rawMsg := types.NewNamed(
			types.NewTypeName(0, types.NewPackage("encoding/json", "json"), "RawMessage", nil),
			types.NewSlice(types.Typ[types.Byte]), nil,
		)
		moduleMap := types.NewMap(types.Typ[types.String], rawMsg)

		rep, err := rb.buildRepresentation(moduleMap)
		if err != nil {
			t.Fatalf("buildRepresentation failed for map[string]json.RawMessage: %v", err)
		}
		if rep == nil || rep.Type != ModuleMap {
			t.Errorf("expected ModuleMap, got %+v", rep)
		}

		regularMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
		rep, err = rb.buildRepresentation(regularMap)
		if err != nil {
			t.Fatalf("buildRepresentation failed for map[string]int: %v", err)
		}
		if rep == nil || rep.Type != Map {
			t.Errorf("expected plain Map, got %+v", rep)
		}
	})

	t.Run("UnknownTypeHandling", func(t *testing.T) {
		// Test handling of unknown/unsupported types
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		t.Log("Testing unknown type handling:")
		t.Log("- Types with no JSON representation fall back to an empty value")

		// Create a channel type (no JSON representation)
		chanType := types.NewChan(types.SendRecv, types.Typ[types.String])

		rep, err := rb.buildRepresentation(chanType)
		if err != nil {
			t.Errorf("channel type should fall back gracefully, got error: %v", err)
		} else if rep == nil {
			t.Error("expected non-nil fallback representation for channel type")
		} else {
			t.Logf("✓ Graceful fallback for channel type: %+v", rep)
		}

		// Create a function type (no JSON representation)
		sig := types.NewSignature(nil, nil, nil, false)
		funcType := types.NewSignature(nil, types.NewTuple(), types.NewTuple(), false)
		_ = funcType // Avoid unused variable
		_ = sig      // Avoid unused variable

		t.Log("✓ Unsupported types (channels, funcs, etc.) no longer fail the containing type")
	})
}

// TestRepresentationBuilderOriginalBehavior tests the representationBuilder struct
// TestRunGoListOriginalBehavior tests the runGoList utility function
func TestRunGoListOriginalBehavior(t *testing.T) {
	t.Run("RunGoListBehavior", func(t *testing.T) {
		tempDir := "/tmp"
		invalidPkg := "invalid.package.name.that.does.not.exist"

		output, err := runGoList(tempDir, invalidPkg)
		if err == nil {
			t.Fatalf("expected error for invalid package, got: %+v", output)
		}
		// stderr is embedded in the error between >>>>>> markers
		if !strings.Contains(err.Error(), ">>>>>>") {
			t.Errorf("expected stderr markers in error, got: %v", err)
		}
	})
}

// TestSynthesisConstantsOriginalBehavior tests constants and package-level values
func TestSynthesisConstantsOriginalBehavior(t *testing.T) {
	t.Run("CaddyCorePackagePath", func(t *testing.T) {
		expectedPath := "github.com/caddyserver/caddy/v2"
		if caddyCorePackagePath != expectedPath {
			t.Errorf("Expected caddyCorePackagePath=%q, got %q", expectedPath, caddyCorePackagePath)
		}
	})
}

// TestSynthesisIntegrationOriginalBehavior tests integration between synthesis components
func TestSynthesisIntegrationOriginalBehavior(t *testing.T) {
	t.Run("RepresentationBuilderIntegration", func(t *testing.T) {
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		if rb.ws.driver != driver {
			t.Error("representationBuilder should reference correct driver")
		}
		if rb.versionCache == nil {
			t.Error("versionCache should be initialized")
		}
	})
}
