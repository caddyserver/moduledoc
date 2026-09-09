package moduledoc

import (
	"strings"
	"testing"
)

func newConfigTree() (*memStorage, *Value) {
	db := newMemStorage()

	ns := "http.handlers"
	ik := "handler"

	root := &Value{
		Type:     Struct,
		TypeName: "example.com/pkg.Config",
		StructFields: []*StructField{
			{
				Key: "listen",
				Value: &Value{
					Type: String,
				},
				Doc: "listen doc",
			},
			{
				Key: "servers",
				Value: &Value{
					Type:    Map,
					MapKeys: &Value{Type: String},
					Elems: &Value{
						Type:     Struct,
						TypeName: "example.com/pkg.Server",
						StructFields: []*StructField{
							{Key: "port", Value: &Value{Type: Int}},
						},
					},
				},
			},
			{
				Key: "handlers",
				Value: &Value{
					Type: Array,
					Elems: &Value{
						Type:            Module,
						ModuleNamespace: &ns,
						ModuleInlineKey: &ik,
					},
				},
			},
		},
	}

	db.modules["http.handlers.file_server"] = []*Value{
		{
			Type:     Struct,
			TypeName: "example.com/fileserver.FileServer",
			StructFields: []*StructField{
				{Key: "root", Value: &Value{Type: String}},
			},
		},
	}

	return db, root
}

func TestTraverseTypeStartValidation(t *testing.T) {
	d := New(newMemStorage())
	if _, _, err := d.TraverseType("a/b", &Value{}); err == nil {
		t.Error("expected error when starting from an untyped value")
	}
}

func TestTraverseTypeEmptyPathReturnsStart(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)
	val, nearest, err := d.TraverseType("", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != root || nearest != root {
		t.Error("empty path should return the start value as both results")
	}
}

func TestTraverseTypeStructField(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)

	val, nearest, err := d.TraverseType("listen", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Type != String {
		t.Errorf("expected string value at path, got %#v", val)
	}
	if !strings.Contains(val.Doc, "listen doc") {
		t.Errorf("struct field doc should be included at target, got %q", val.Doc)
	}
	if nearest.TypeName != "example.com/pkg.Config" {
		t.Errorf("nearest type should be the containing config type, got %q", nearest.TypeName)
	}
}

func TestTraverseTypeUnknownStructField(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)
	if _, _, err := d.TraverseType("nope", root); err == nil {
		t.Error("expected error for unknown struct field")
	}
}

func TestTraverseTypeThroughMapContainer(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)

	// maps and arrays are transparent containers: the next path
	// part applies to the element type, not a key or index
	val, nearest, err := d.TraverseType("servers/port", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Type != Int {
		t.Errorf("expected int value at path, got %#v", val)
	}
	if nearest.TypeName != "example.com/pkg.Server" {
		t.Errorf("nearest type should be the map's element type, got %q", nearest.TypeName)
	}
}

func TestTraverseTypeModuleLookup(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)

	val, _, err := d.TraverseType("handlers/file_server", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.TypeName != "example.com/fileserver.FileServer" {
		t.Errorf("expected module type at path, got %#v", val)
	}
	if val.ModuleInlineKey == nil || *val.ModuleInlineKey != "handler" {
		t.Errorf("inline key should be set on module resolved at the final path part, got %#v", val.ModuleInlineKey)
	}

	// traversal into the module's own fields
	val, _, err = d.TraverseType("handlers/file_server/root", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Type != String {
		t.Errorf("expected string value inside module, got %#v", val)
	}
}

func TestTraverseTypeUnknownModuleID(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("traversal to an unknown module ID must return an error, not panic: %v", r)
		}
	}()
	if _, _, err := d.TraverseType("handlers/does_not_exist", root); err == nil {
		t.Error("expected error for unknown module ID")
	}
}

func TestTraverseTypeNotTraversable(t *testing.T) {
	db, root := newConfigTree()
	d := New(db)
	if _, _, err := d.TraverseType("listen/deeper", root); err == nil {
		t.Error("expected error when traversing into a primitive value")
	}
}
