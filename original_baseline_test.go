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
	"os"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestOriginalImplementation tests the existing functionality to establish a baseline
func TestOriginalImplementation(t *testing.T) {
	t.Run("BasicModuleDetection", func(t *testing.T) {
		// Test that basic module detection works with static string literals
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
			t.Fatalf("Failed to load testdata: %v", err)
		}

		if len(pkgs) == 0 {
			t.Fatal("No packages loaded")
		}

		driver := New(nil)
		pkg := pkgs[0]

		// Test findCaddyModuleIdents with the original gizmo.go
		moduleIdents, err := driver.findCaddyModuleIdents(pkg)
		if err != nil {
			t.Errorf("findCaddyModuleIdents failed: %v", err)
		}

		// Should find the static module from gizmo.go
		expectedModuleID := "app.namespace.gizmo"
		found := false
		for _, moduleID := range moduleIdents {
			if moduleID == expectedModuleID {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected to find module ID '%s' but got: %v", expectedModuleID, moduleIdents)
		}

		t.Logf("Successfully found %d modules: %v", len(moduleIdents), moduleIdents)
	})
}

// TestOriginalCachingBehavior tests the caching mechanisms in the original implementation
func TestOriginalCachingBehavior(t *testing.T) {
	t.Run("UnboundedCacheGrowth", func(t *testing.T) {
		// Test that demonstrates unbounded cache growth
		driver := New(nil)

		// Simulate adding many entries to the cache
		for i := 0; i < 1000; i++ {
			key := "test.type." + string(rune(i))
			driver.setDiscoveredType(key, &Value{
				Type:     String,
				TypeName: key,
			})
		}

		// no size limit, TTL, or eviction: every entry is retained
		if len(driver.discoveredTypes) != 1000 {
			t.Errorf("Expected 1000 cache entries, got %d", len(driver.discoveredTypes))
		}
	})
}

// TestOriginalErrorHandling tests error handling patterns in the original code
func TestOriginalErrorHandling(t *testing.T) {
	t.Run("StrictModuleValidation", func(t *testing.T) {
		// registration without a local CaddyModule method fails the package
		source := `
package test

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(HalfModule))
}

type HalfModule struct{}
`
		testModuleSource(t, source, false, "Registration without implementation should fail")
	})
}
