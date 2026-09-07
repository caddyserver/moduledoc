// Copyright 2019 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package moduledoc

import (
	"go/types"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadAliasesFixture loads the testdata/aliases fixture package and returns it.
func loadAliasesFixture(t *testing.T) *packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedModule,
		Dir: "testdata/aliases",
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		t.Fatalf("package has errors: %v", pkg.Errors)
	}
	return pkg
}

// lookupType returns the top-level type identified by name in pkg.
func lookupType(t *testing.T, pkg *packages.Package, name string) types.Type {
	t.Helper()
	obj := pkg.Types.Scope().Lookup(name)
	if obj == nil {
		t.Fatalf("type %q not found in %s", name, pkg.PkgPath)
	}
	return obj.Type()
}

// newTestBuilder returns a representationBuilder wired to the given fixture,
// with the workspace's package cache and the builder's version cache
// preloaded so no real 'go get' / 'go list' invocations are attempted.
func newTestBuilder(t *testing.T, pkg *packages.Package) representationBuilder {
	t.Helper()
	drv := New(newMemStorage())
	ws := workspace{
		mu:              new(sync.RWMutex),
		dir:             t.TempDir(),
		driver:          drv,
		goGets:          map[string]struct{}{pkg.PkgPath: {}},
		packagePatterns: map[string][]string{},
		parsedPackages:  map[string]*packages.Package{pkg.PkgPath: pkg, pkg.ID: pkg},
	}
	rb := ws.representationBuilder()
	rb.versionCache[pkg.PkgPath] = ""
	return rb
}
