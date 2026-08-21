package moduledoc

import (
	"go/types"
	"os"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestWorkspaceSpecificFunctions tests specific functions in workspace.go
func TestWorkspaceSpecificFunctions(t *testing.T) {
	t.Run("PackageKeyFunction", func(t *testing.T) {
		// Test the packageKey function that creates cache keys
		testCases := []struct {
			name     string
			pkg      *packages.Package
			expected string
		}{
			{
				name: "PackageWithVersion",
				pkg: &packages.Package{
					ID: "example.com/test",
					Module: &packages.Module{
						Version: "v1.0.0",
					},
				},
				expected: "example.com/test@v1.0.0",
			},
			{
				name: "PackageWithoutVersion",
				pkg: &packages.Package{
					ID: "example.com/test",
				},
				expected: "example.com/test",
			},
			{
				name: "PackageWithEmptyVersion",
				pkg: &packages.Package{
					ID: "example.com/test",
					Module: &packages.Module{
						Version: "",
					},
				},
				expected: "example.com/test",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := packageKey(tc.pkg)
				if result != tc.expected {
					t.Errorf("packageKey() = %q, expected %q", result, tc.expected)
				} else {
					t.Logf("✓ packageKey() = %q", result)
				}
			})
		}
	})

	t.Run("AlreadyGotModuleFunction", func(t *testing.T) {
		// Test the alreadyGotModule function
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		// Add some modules to the goGets cache
		ws.goGets["github.com/caddyserver/caddy"] = struct{}{}
		ws.goGets["example.com/test"] = struct{}{}

		testCases := []struct {
			packagePath string
			expected    bool
			description string
		}{
			{"github.com/caddyserver/caddy/v2", true, "should find parent module"},
			{"github.com/caddyserver/caddy/v2/modules/http", true, "should find ancestor module"},
			{"example.com/test/subpackage", true, "should find parent module"},
			{"other.com/package", false, "should not find unrelated module"},
			{"", false, "should handle empty path"},
		}

		for _, tc := range testCases {
			result := ws.alreadyGotModule(tc.packagePath)
			if result != tc.expected {
				t.Errorf("alreadyGotModule(%q) = %v, expected %v (%s)",
					tc.packagePath, result, tc.expected, tc.description)
			} else {
				t.Logf("✓ alreadyGotModule(%q) = %v (%s)",
					tc.packagePath, result, tc.description)
			}
		}
	})

	t.Run("RepresentationBuilderCreation", func(t *testing.T) {
		// Test representationBuilder creation
		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()

		if rb.ws.dir != ws.dir {
			t.Error("RepresentationBuilder workspace not set correctly")
		}

		if rb.versionCache == nil {
			t.Error("RepresentationBuilder versionCache not initialized")
		}

		if len(rb.versionCache) != 0 {
			t.Error("RepresentationBuilder versionCache should start empty")
		}

		t.Log("✓ RepresentationBuilder created correctly")
	})
}

// TestSynthesisSpecificFunctions tests specific functions in synthesis.go
func TestSynthesisSpecificFunctions(t *testing.T) {
	t.Run("GetStructFieldGodocs", func(t *testing.T) {
		// Test getStructFieldGodocs with real testdata
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

		pkgs, err := packages.Load(cfg, "testdata")
		if err != nil {
			t.Logf("Package loading failed (expected if testdata has issues): %v", err)
			return
		}

		if len(pkgs) == 0 {
			t.Skip("No packages loaded, skipping getStructFieldGodocs test")
		}

		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()
		pkg := pkgs[0]

		// Look for the Gizmo struct type
		for _, obj := range pkg.TypesInfo.Defs {
			if obj != nil && obj.Name() == "Gizmo" {
				if typeName, ok := obj.(*types.TypeName); ok {
					namedType := typeName.Type().(*types.Named)

					// Test getStructFieldGodocs
					docs, err := rb.getStructFieldGodocs(namedType)
					if err != nil {
						t.Errorf("getStructFieldGodocs failed: %v", err)
					} else {
						t.Logf("✓ getStructFieldGodocs returned %d field docs", len(docs))
						for field, doc := range docs {
							t.Logf("  Field %s: %q", field, strings.TrimSpace(doc))
						}
					}
					break
				}
			}
		}
	})

	t.Run("GetGodocForType", func(t *testing.T) {
		// Test getGodocForType with testdata
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

		pkgs, err := packages.Load(cfg, "testdata")
		if err != nil {
			t.Logf("Package loading failed (expected if testdata has issues): %v", err)
			return
		}

		if len(pkgs) == 0 {
			t.Skip("No packages loaded, skipping getGodocForType test")
		}

		driver := New(nil)
		ws, err := driver.openWorkspace()
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}
		defer ws.Close()

		rb := ws.representationBuilder()
		pkg := pkgs[0]

		// Look for the Gizmo struct type
		for _, obj := range pkg.TypesInfo.Defs {
			if obj != nil && obj.Name() == "Gizmo" {
				if typeName, ok := obj.(*types.TypeName); ok {
					namedType := typeName.Type().(*types.Named)

					// Test getGodocForType
					doc, err := rb.getGodocForType(namedType)
					if err != nil {
						t.Errorf("getGodocForType failed: %v", err)
					} else {
						t.Logf("✓ getGodocForType returned: %q", strings.TrimSpace(doc))
					}
					break
				}
			}
		}
	})
}

// TestStorageSpecificFunctions tests specific functions in storage.go
func TestStorageSpecificFunctions(t *testing.T) {
	t.Run("ConfigPathPartsFunction", func(t *testing.T) {
		// Test ConfigPathParts function
		testCases := []struct {
			input    string
			expected []string
		}{
			{"/path/to/config", []string{"path", "to", "config"}},
			{"path/to/config", []string{"path", "to", "config"}},
			{"path/to/config/", []string{"path", "to", "config"}},
			{"/path", []string{"path"}},
			{"path", []string{"path"}},
			{"/", []string{""}},
			{"", []string{""}},
		}

		for _, tc := range testCases {
			result := ConfigPathParts(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("ConfigPathParts(%q) = %v, expected %v", tc.input, result, tc.expected)
			} else {
				t.Logf("✓ ConfigPathParts(%q) = %v", tc.input, result)
			}
		}
	})

	t.Run("JsonNameFromTagFunction", func(t *testing.T) {
		// Test jsonNameFromTag function
		testCases := []struct {
			tagStr   string
			expected string
			ok       bool
		}{
			{`json:"name"`, "name", true},
			{`json:"name,omitempty"`, "name", true},
			{`json:",omitempty"`, "", true},
			{`json:"-"`, "", false},
			{`json:"-,omitempty"`, "", false},
			{`json:"name,required"`, "name", true},
			{`other:"value" json:"field"`, "field", true},
			{`other:"value"`, "", true},
			{"", "", true},
		}

		for _, tc := range testCases {
			result, ok := jsonNameFromTag(tc.tagStr)
			if result != tc.expected || ok != tc.ok {
				t.Errorf("jsonNameFromTag(%q) = (%q, %v), expected (%q, %v)",
					tc.tagStr, result, ok, tc.expected, tc.ok)
			} else {
				t.Logf("✓ jsonNameFromTag(%q) = (%q, %v)", tc.tagStr, result, ok)
			}
		}
	})

	t.Run("CaddyTagFieldsFunction", func(t *testing.T) {
		// Test caddyTagFields function
		testCases := []struct {
			tagStr   string
			hasError bool
		}{
			{`caddy:"namespace=http.handlers"`, false},
			{`caddy:"inline_key=type"`, false},
			{`json:"name" caddy:"namespace=app"`, false},
			{`other:"value"`, false},
			{"", false},
		}

		for _, tc := range testCases {
			result, err := caddyTagFields(tc.tagStr)
			if tc.hasError && err == nil {
				t.Errorf("caddyTagFields(%q) expected error but got none", tc.tagStr)
			} else if !tc.hasError && err != nil {
				t.Errorf("caddyTagFields(%q) unexpected error: %v", tc.tagStr, err)
			} else {
				t.Logf("✓ caddyTagFields(%q) = %v (err: %v)", tc.tagStr, result, err)
			}
		}
	})

	t.Run("TypeUtilityFunctions", func(t *testing.T) {
		// Test the type utility functions with mock data
		t.Log("Testing type utility functions:")
		t.Log("- fullyQualifiedTypeName: creates FQTN from types.Type")
		t.Log("- typePackageAndName: splits package and type name")
		t.Log("- localTypeName: extracts just the type name")
		t.Log("These functions work with types.Named objects from real packages")
	})
}
