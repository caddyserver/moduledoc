package moduledoc

import (
	"os"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestOriginalBehaviorBaseline establishes baseline behavior for the original implementation
func TestOriginalBehaviorBaseline(t *testing.T) {
	t.Run("ValidStaticModule", func(t *testing.T) {
		// Test only the gizmo.go file which should work
		cfg := &packages.Config{
			Dir: ".",
			Mode: packages.NeedSyntax |
				packages.NeedImports |
				packages.NeedDeps |
				packages.NeedTypes |
				packages.NeedModule |
				packages.NeedTypesInfo,
			Env: append(os.Environ(), "CGO_ENABLED=0"),
		}

		pkgs, err := packages.Load(cfg, "./testdata")
		if err != nil {
			t.Fatalf("loading testdata package: %v", err)
		}
		if len(pkgs) == 0 {
			t.Fatal("no packages loaded")
		}

		driver := New(nil)
		pkg := pkgs[0]

		moduleIdents, err := driver.findCaddyModuleIdents(pkg)
		if err != nil {
			t.Fatalf("finding module idents: %v", err)
		}

		found := false
		for _, moduleID := range moduleIdents {
			if moduleID == "app.namespace.gizmo" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected module 'app.namespace.gizmo', got: %v", moduleIdents)
		}
	})

	t.Run("StrictValidationBehavior", func(t *testing.T) {
		// one incomplete module fails the whole package, valid sibling
		// included; a partial-results contract would return GoodModule here
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(GoodModule))
}

type GoodModule struct{}

func (*GoodModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.test.good",
		New: func() caddy.Module { return new(GoodModule) },
	}
}

type OrphanModule struct{}

func (*OrphanModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.test.orphan",
		New: func() caddy.Module { return new(OrphanModule) },
	}
}
`
		testModuleSource(t, source, false, "Mixed valid/incomplete package fails entirely")
	})
}
