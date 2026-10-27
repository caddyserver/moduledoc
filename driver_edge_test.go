package moduledoc

import (
	"strings"
	"testing"
)

func TestLoadTypesByModuleID(t *testing.T) {
	db := newMemStorage()
	db.types[storageKey("example.com/pkg", "Handler", "")] = &Value{
		Type:     Struct,
		TypeName: "example.com/pkg.Handler",
		Doc:      "handler doc",
		StructFields: []*StructField{
			{Key: "root", Value: &Value{Type: String}, Doc: "root doc"},
		},
	}
	db.modules["http.handlers.x"] = []*Value{
		{SameAs: "example.com/pkg.Handler", Doc: "usage doc"},
	}
	d := New(db)

	vals, err := d.LoadTypesByModuleID("http.handlers.x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 1 {
		t.Fatalf("expected 1 value, got %d", len(vals))
	}
	got := vals[0]
	if got.Type != Struct || got.TypeName != "example.com/pkg.Handler" {
		t.Errorf("module type not resolved, got %+v", got)
	}
	if !strings.Contains(got.Doc, "usage doc") || !strings.Contains(got.Doc, "handler doc") {
		t.Errorf("docs should combine usage and type docs, got %q", got.Doc)
	}
	if len(got.StructFields) != 1 || got.StructFields[0].Value.Type != String {
		t.Errorf("struct fields not deeply dereferenced: %+v", got.StructFields)
	}

	// unknown IDs yield an empty result, not an error
	vals, err = d.LoadTypesByModuleID("does.not.exist")
	if err != nil {
		t.Fatalf("unexpected error for unknown module ID: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected no values for unknown module ID, got %d", len(vals))
	}
}

func TestLoadTypeByPath(t *testing.T) {
	db := newMemStorage()
	db.types[storageKey(CaddyCorePackage, "Config", "")] = &Value{
		Type:     Struct,
		TypeName: CaddyCorePackage + ".Config",
		StructFields: []*StructField{
			{
				Key:   "listen",
				Value: &Value{SameAs: "example.com/pkg.Listen"},
				Doc:   "listen field doc",
			},
		},
	}
	db.types[storageKey("example.com/pkg", "Listen", "")] = &Value{
		Type:     String,
		TypeName: "example.com/pkg.Listen",
		Doc:      "listen type doc",
	}
	d := New(db)

	exact, nearest, err := d.LoadTypeByPath("listen", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exact == nil || exact.Type != String {
		t.Errorf("expected resolved string type at path, got %+v", exact)
	}
	if !strings.Contains(exact.Doc, "listen field doc") {
		t.Errorf("field doc should be included at target, got %q", exact.Doc)
	}
	if nearest == nil || nearest.TypeName == "" {
		t.Errorf("expected a named nearest type, got %+v", nearest)
	}

	// without a stored Config start type, the lookup must error
	if _, _, err := New(newMemStorage()).LoadTypeByPath("listen", ""); err == nil {
		t.Error("expected error when start type is missing")
	}
}

func TestAddTypeStdlibStruct(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	db := newMemStorage()
	d := New(db)

	// stdlib packages resolve offline in a fresh workspace
	rep, err := d.AddType("net/url", "URL", "")
	if err != nil {
		t.Fatalf("AddType failed: %v", err)
	}
	if !strings.Contains(rep.SameAs, "net/url.URL") {
		t.Errorf("expected reference to net/url.URL, got %+v", rep)
	}

	stored, err := db.GetTypeByName("net/url", "URL", "")
	if err != nil || stored == nil {
		t.Fatalf("stored type not found: %v", err)
	}
	if stored.Type != Struct {
		t.Errorf("expected struct, got %s", stored.Type)
	}
	if !strings.Contains(stored.Doc, "URL") {
		t.Errorf("expected type godoc, got %q", stored.Doc)
	}
}

func TestAddTypeErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	d := New(newMemStorage())

	if _, err := d.AddType("net/url", "DoesNotExist", ""); err == nil {
		t.Error("expected error for unknown type name")
	}
}

func TestLoadModulesFromImportingPackageEmptyPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	d := New(newMemStorage())
	if _, err := d.LoadModulesFromImportingPackage("", ""); err == nil {
		t.Error("expected error for empty package pattern")
	}
}
