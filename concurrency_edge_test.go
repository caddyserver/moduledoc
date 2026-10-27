package moduledoc

import (
	"os"
	"os/exec"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// localWorkspace returns a workspace anchored at the repository root so
// the local testdata package can be loaded without running 'go get'.
func localWorkspace(t *testing.T, d *Driver) workspace {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	return workspace{
		mu:              new(sync.RWMutex),
		dir:             cwd,
		driver:          d,
		goGets:          map[string]struct{}{"github.com/caddyserver/moduledoc": {}},
		packagePatterns: make(map[string][]string),
		parsedPackages:  make(map[string]*packages.Package),
	}
}

const testdataPackagePath = "github.com/caddyserver/moduledoc/testdata"

// runIsolated re-runs the named test in a child process with childEnv set,
// so fatal runtime errors or race reports cannot take down the whole suite.
func runIsolated(t *testing.T, testName, childEnv, failureMsg string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "^"+testName+"$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 4000 {
			out = out[:4000]
		}
		t.Fatalf("%s: %v\n%s", failureMsg, err, out)
	}
}

func TestConcurrentDiscoveredTypesAccess(t *testing.T) {
	if os.Getenv("MODULEDOC_TEST_DISCOVERED_TYPES_CHILD") == "1" {
		db := newMemStorage()
		d := New(db)
		ws := localWorkspace(t, d)

		pkgs, err := ws.getPackages(testdataPackagePath, "")
		if err != nil {
			t.Fatalf("loading testdata package: %v", err)
		}
		obj := pkgs[0].Types.Scope().Lookup("Gizmo")
		if obj == nil {
			t.Fatal("Gizmo type not found in testdata package")
		}

		// pre-store the type so every build takes the db-hit path,
		// which writes to the driver's type cache on each call
		db.StoreType(testdataPackagePath, "Gizmo", "", &Value{
			Type:     Struct,
			TypeName: testdataPackagePath + ".Gizmo",
		})

		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				rb := ws.representationBuilder()
				if _, err := rb.buildRepresentation(obj.Type()); err != nil {
					t.Errorf("building representation: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
		return
	}

	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	runIsolated(t, "TestConcurrentDiscoveredTypesAccess", "MODULEDOC_TEST_DISCOVERED_TYPES_CHILD",
		"concurrent representation building on a shared Driver must be safe")
}

func TestConcurrentModuleTypeLoading(t *testing.T) {
	if os.Getenv("MODULEDOC_TEST_MODULE_LOADING_CHILD") == "1" {
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

		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if _, err := d.LoadTypesByModuleID("http.handlers.x"); err != nil {
					t.Errorf("loading module types: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
		return
	}

	runIsolated(t, "TestConcurrentModuleTypeLoading", "MODULEDOC_TEST_MODULE_LOADING_CHILD",
		"concurrent module type loading on a shared Driver must be safe")
}

func TestConcurrentGetPackages(t *testing.T) {
	if os.Getenv("MODULEDOC_TEST_GET_PACKAGES_CHILD") == "1" {
		d := New(newMemStorage())
		ws := localWorkspace(t, d)

		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				pkgs, err := ws.getPackages(testdataPackagePath, "")
				if err != nil {
					t.Errorf("loading packages: %v", err)
					return
				}
				if len(pkgs) != 1 {
					t.Errorf("expected 1 package, got %d", len(pkgs))
				}
			}()
		}
		close(start)
		wg.Wait()
		return
	}

	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	runIsolated(t, "TestConcurrentGetPackages", "MODULEDOC_TEST_GET_PACKAGES_CHILD",
		"concurrent package loading on a shared workspace must be safe")
}
