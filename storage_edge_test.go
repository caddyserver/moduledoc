package moduledoc

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// memStorage is a minimal thread-safe in-memory Storage for unit tests.
type memStorage struct {
	mu      sync.Mutex
	types   map[string]*Value
	modules map[string][]*Value
}

func newMemStorage() *memStorage {
	return &memStorage{
		types:   make(map[string]*Value),
		modules: make(map[string][]*Value),
	}
}

func storageKey(packagePath, name, version string) string {
	return packagePath + "." + name + "@" + version
}

func (m *memStorage) GetTypeByName(packagePath, name, version string) (*Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.types[storageKey(packagePath, name, version)], nil
}

func (m *memStorage) GetTypesByCaddyModuleID(caddyModuleID string) ([]*Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// return a fresh slice, as a database-backed implementation would
	return append([]*Value(nil), m.modules[caddyModuleID]...), nil
}

func (m *memStorage) StoreType(packagePath, typeName, version string, rep *Value) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.types[storageKey(packagePath, typeName, version)] = rep
	return nil
}

func (m *memStorage) SetCaddyModuleName(pkg *packages.Package, typeName, modName string) error {
	return nil
}

func TestDereferenceNoOpWithoutSameAs(t *testing.T) {
	d := New(newMemStorage())
	val := &Value{Type: String, Doc: "a doc"}
	got, err := d.dereference(val)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != val {
		t.Errorf("expected same value back when SameAs is empty")
	}
}

func TestDereferenceTypeNotFound(t *testing.T) {
	d := New(newMemStorage())

	_, err := d.dereference(&Value{SameAs: "example.com/pkg.Missing"})
	if err == nil {
		t.Error("expected error for unknown referenced type, got nil")
	}

	// malformed reference without any dot must error, not panic
	_, err = d.dereference(&Value{SameAs: "nodot"})
	if err == nil {
		t.Error("expected error for malformed type reference, got nil")
	}
}

func TestDereferenceVersionedReference(t *testing.T) {
	db := newMemStorage()
	db.types[storageKey("example.com/pkg", "T", "v1.2.3")] = &Value{Type: String, TypeName: "example.com/pkg.T"}
	d := New(db)

	got, err := d.dereference(&Value{SameAs: "example.com/pkg.T@v1.2.3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != String {
		t.Errorf("expected resolved string type, got %#v", got)
	}

	// same reference without the version must not resolve to the versioned entry
	if _, err := d.dereference(&Value{SameAs: "example.com/pkg.T"}); err == nil {
		t.Error("expected error for unversioned reference to a versioned-only type")
	}
}

func TestDereferenceTransfersModuleInfoToInnermostElem(t *testing.T) {
	db := newMemStorage()
	// stored shape: array of maps of modules
	db.types[storageKey("example.com/pkg", "Handlers", "")] = &Value{
		Type:     Array,
		TypeName: "example.com/pkg.Handlers",
		Elems: &Value{
			Type:  Map,
			Elems: &Value{Type: Module},
		},
	}
	d := New(db)

	ns := "http.handlers"
	ik := "handler"
	got, err := d.dereference(&Value{
		SameAs:          "example.com/pkg.Handlers",
		ModuleNamespace: &ns,
		ModuleInlineKey: &ik,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inner := got
	for inner.Elems != nil {
		inner = inner.Elems
	}
	if inner.ModuleNamespace == nil || *inner.ModuleNamespace != ns {
		t.Errorf("module namespace not transferred to innermost element: %#v", inner)
	}
	if inner.ModuleInlineKey == nil || *inner.ModuleInlineKey != ik {
		t.Errorf("module inline key not transferred to innermost element: %#v", inner)
	}
	if got.ModuleNamespace != nil && got.Elems != nil {
		t.Errorf("namespace should live on the innermost element only")
	}
}

func TestDereferenceDoesNotMutateStoredType(t *testing.T) {
	db := newMemStorage()
	db.types[storageKey("example.com/pkg", "T", "")] = &Value{
		Type:     String,
		TypeName: "example.com/pkg.T",
		Doc:      "type doc",
	}
	d := New(db)

	ns := "http.handlers"
	first, err := d.dereference(&Value{SameAs: "example.com/pkg.T", Doc: "field doc", ModuleNamespace: &ns})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantDoc := "field doc\n\ntype doc"
	if first.Doc != wantDoc {
		t.Errorf("first dereference doc = %q; want %q", first.Doc, wantDoc)
	}

	// dereferencing the same reference again from a different context
	// must not accumulate docs or leak module info between contexts
	second, err := d.dereference(&Value{SameAs: "example.com/pkg.T", Doc: "field doc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Doc != wantDoc {
		t.Errorf("second dereference doc = %q; want %q (docs must not accumulate)", second.Doc, wantDoc)
	}
	if second.ModuleNamespace != nil {
		t.Errorf("second dereference has namespace %q leaked from a prior dereference", *second.ModuleNamespace)
	}
	if first.ModuleNamespace == nil || *first.ModuleNamespace != ns {
		t.Errorf("first result's namespace was clobbered by a later dereference: %#v", first.ModuleNamespace)
	}
}

func TestDeepDereferenceResolvesNestedReferences(t *testing.T) {
	db := newMemStorage()
	db.types[storageKey("example.com/pkg", "Leaf", "")] = &Value{
		Type:     String,
		TypeName: "example.com/pkg.Leaf",
		Doc:      "leaf type doc",
	}
	db.types[storageKey("example.com/pkg", "Root", "")] = &Value{
		Type:     Struct,
		TypeName: "example.com/pkg.Root",
		StructFields: []*StructField{
			{Key: "leaf", Value: &Value{SameAs: "example.com/pkg.Leaf"}, Doc: "field doc"},
		},
		MapKeys: &Value{SameAs: "example.com/pkg.Leaf"},
		Elems:   nil,
	}
	d := New(db)

	got, err := d.deepDereference(&Value{SameAs: "example.com/pkg.Root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.StructFields) != 1 {
		t.Fatalf("expected 1 struct field, got %d", len(got.StructFields))
	}
	sf := got.StructFields[0]
	if sf.Value.Type != String {
		t.Errorf("struct field not dereferenced: %#v", sf.Value)
	}
	if !strings.Contains(sf.Doc, "field doc") || !strings.Contains(sf.Doc, "leaf type doc") {
		t.Errorf("field doc should combine field and type docs, got %q", sf.Doc)
	}
	if !strings.HasPrefix(sf.Doc, "field doc") {
		t.Errorf("field doc should lead with the more specific field doc, got %q", sf.Doc)
	}
	if got.MapKeys == nil || got.MapKeys.Type != String {
		t.Errorf("map keys not dereferenced: %#v", got.MapKeys)
	}
}

func TestDeepDereferenceCycle(t *testing.T) {
	// run in a child process because unbounded recursion kills the
	// whole process and would take every other test down with it
	if os.Getenv("MODULEDOC_TEST_CYCLE_CHILD") == "1" {
		db := newMemStorage()
		db.types[storageKey("example.com/pkg", "Node", "")] = &Value{
			Type:     Struct,
			TypeName: "example.com/pkg.Node",
			StructFields: []*StructField{
				{Key: "next", Value: &Value{SameAs: "example.com/pkg.Node"}},
			},
		}
		d := New(db)
		if _, err := d.deepDereference(&Value{SameAs: "example.com/pkg.Node"}); err == nil {
			t.Fatal("expected error for self-referential type, got nil")
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestDeepDereferenceCycle$")
	cmd.Env = append(os.Environ(), "MODULEDOC_TEST_CYCLE_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 2000 {
			out = out[:2000]
		}
		t.Fatalf("deep dereference of a self-referential type must fail gracefully, not crash: %v\n%s", err, out)
	}
}
