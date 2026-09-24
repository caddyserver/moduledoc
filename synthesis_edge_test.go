package moduledoc

import (
	"go/types"
	"os"
	"strings"
	"testing"
)

const kindsPackagePath = "github.com/caddyserver/moduledoc/testdata/kinds"

// kindsBuilder loads the kinds fixture package and returns a builder
// plus the named type, using an in-memory storage on the driver.
func kindsBuilder(t *testing.T, typeName string) (*memStorage, representationBuilder, types.Type) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	db := newMemStorage()
	d := New(db)
	ws := localWorkspace(t, d)
	pkgs, err := ws.getPackages(kindsPackagePath, "")
	if err != nil {
		t.Fatalf("loading kinds package: %v", err)
	}
	obj := pkgs[0].Types.Scope().Lookup(typeName)
	if obj == nil {
		t.Fatalf("type %s not found in kinds package", typeName)
	}
	return db, ws.representationBuilder(), obj.Type()
}

func structField(rep *Value, key string) *StructField {
	for _, sf := range rep.StructFields {
		if sf.Key == key {
			return sf
		}
	}
	return nil
}

func TestBuildRepresentationWidgetFields(t *testing.T) {
	db, rb, typ := kindsBuilder(t, "Widget")

	ref, err := rb.buildRepresentation(typ)
	if err != nil {
		t.Fatalf("building representation: %v", err)
	}
	if ref.SameAs == "" {
		t.Fatalf("expected reference to stored type, got %#v", ref)
	}
	rep, err := db.GetTypeByName(kindsPackagePath, "Widget", "")
	if err != nil || rep == nil {
		t.Fatalf("stored Widget type not found: %v", err)
	}

	if rep.Type != Struct {
		t.Fatalf("expected struct, got %#v", rep.Type)
	}
	if !strings.Contains(rep.Doc, "exercises the field kinds") {
		t.Errorf("type godoc missing, got %q", rep.Doc)
	}

	if sf := structField(rep, "name"); sf == nil || sf.Value.Type != String {
		t.Errorf("expected string field 'name', got %#v", sf)
	} else if !strings.Contains(sf.Doc, "Name is the widget name") {
		t.Errorf("field godoc missing, got %q", sf.Doc)
	}

	for _, absent := range []string{"Hidden", "-", "unexported"} {
		if sf := structField(rep, absent); sf != nil {
			t.Errorf("field %q should not be documented", absent)
		}
	}

	if sf := structField(rep, "double_ptr"); sf == nil || sf.Value.Type != Int {
		t.Errorf("pointer-to-pointer field should unwrap to int, got %#v", sf)
	}
	if sf := structField(rep, "numbers"); sf == nil || sf.Value.Type != Array || sf.Value.Elems == nil || sf.Value.Elems.Type != Int {
		t.Errorf("expected array of int, got %#v", sf)
	}
	if sf := structField(rep, "lookup"); sf == nil || sf.Value.Type != Map ||
		sf.Value.MapKeys == nil || sf.Value.MapKeys.Type != Int ||
		sf.Value.Elems == nil || sf.Value.Elems.Type != String {
		t.Errorf("expected map with int keys and string values, got %#v", sf)
	}
	if sf := structField(rep, "anything"); sf == nil || sf.Value.Type != "" {
		t.Errorf("interface field should have an empty representation, got %#v", sf)
	}

	if sf := structField(rep, "raw"); sf == nil || sf.Value.Type != Module {
		t.Errorf("json.RawMessage field should be a module, got %#v", sf)
	} else {
		if sf.Value.ModuleNamespace == nil || *sf.Value.ModuleNamespace != "widget.raw" {
			t.Errorf("module namespace not applied, got %#v", sf.Value.ModuleNamespace)
		}
		if sf.Value.ModuleInlineKey == nil || *sf.Value.ModuleInlineKey != "kind" {
			t.Errorf("module inline key not applied, got %#v", sf.Value.ModuleInlineKey)
		}
	}
	if sf := structField(rep, "raw_map"); sf == nil || sf.Value.Type != ModuleMap {
		t.Errorf("map of json.RawMessage should be a module map, got %#v", sf)
	} else if sf.Value.ModuleNamespace == nil || *sf.Value.ModuleNamespace != "widget.rawmap" {
		t.Errorf("module map namespace not applied, got %#v", sf.Value.ModuleNamespace)
	}

	if sf := structField(rep, "extra"); sf == nil || sf.Value.Type != Bool {
		t.Errorf("embedded struct fields should be promoted, got %#v", sf)
	}
	if sf := structField(rep, "Embedded"); sf != nil {
		t.Error("embedded type itself should not appear as a field")
	}

	if sf := structField(rep, "nested"); sf == nil || !strings.Contains(sf.Value.SameAs, "kinds.Nested") {
		t.Errorf("named nested type should be stored by reference, got %#v", sf)
	}
	if nested, _ := db.GetTypeByName(kindsPackagePath, "Nested", ""); nested == nil {
		t.Error("nested named type should have been stored")
	}

	if sf := structField(rep, "inline"); sf == nil || sf.Value.Type != Struct || structField(sf.Value, "a") == nil {
		t.Errorf("inline anonymous struct should be represented with its fields, got %#v", sf)
	}
}

func TestBuildRepresentationUnsupportedFieldTypes(t *testing.T) {
	db, rb, typ := kindsBuilder(t, "Tricky")

	// function- and channel-typed fields cannot appear in JSON config,
	// but they must not prevent documenting the rest of the type
	if _, err := rb.buildRepresentation(typ); err != nil {
		t.Fatalf("expected graceful handling of func/chan fields, got error: %v", err)
	}
	rep, err := db.GetTypeByName(kindsPackagePath, "Tricky", "")
	if err != nil || rep == nil {
		t.Fatalf("stored Tricky type not found: %v", err)
	}
}

func TestBuildRepresentationRecursiveType(t *testing.T) {
	if os.Getenv("MODULEDOC_TEST_RECURSIVE_TYPE_CHILD") == "1" {
		_, rb, typ := kindsBuilder(t, "Node")
		// success or a clean error are both fine; a crash is not
		if _, err := rb.buildRepresentation(typ); err != nil {
			t.Logf("recursive type produced error: %v", err)
		}
		return
	}
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	runIsolated(t, "TestBuildRepresentationRecursiveType", "MODULEDOC_TEST_RECURSIVE_TYPE_CHILD",
		"a self-referential struct type must not crash representation building")
}
