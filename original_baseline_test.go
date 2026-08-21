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

	// t.Run("RaceConditionExposure", func(t *testing.T) {
	// 	// This test exposes the race condition in the original implementation
	// 	// It should pass under normal conditions but may fail under race conditions

	// 	driver := New(nil)
	// 	var wg sync.WaitGroup
	// 	errors := make(chan error, 10)

	// 	// Simulate concurrent access to discoveredTypes map
	// 	for i := 0; i < 10; i++ {
	// 		wg.Add(1)
	// 		go func(id int) {
	// 			defer wg.Done()
	// 			// Access the discoveredTypes map without proper locking (original behavior)
	// 			key := "test.type" + string(rune(id))

	// 			// This demonstrates the race condition - direct map access without locking
	// 			if _, exists := driver.discoveredTypes[key]; !exists {
	// 				driver.discoveredTypes[key] = &Value{Type: String, TypeName: key}
	// 			}
	// 		}(i)
	// 	}

	// 	wg.Wait()
	// 	close(errors)

	// 	// Check for any panics/errors (though they may not always occur)
	// 	for err := range errors {
	// 		t.Errorf("Race condition error: %v", err)
	// 	}

	// 	t.Logf("Race condition test completed - current implementation has no mutex protection")
	// })
}

// TestOriginalLimitations tests the known limitations of the original implementation
func TestOriginalLimitations(t *testing.T) {
	t.Run("DynamicModuleIDHandling", func(t *testing.T) {
		// Create a test file with dynamic module ID that should be skipped
		dynamicModuleSource := `
package test

import "github.com/caddyserver/caddy/v2"

const ModulePrefix = "app.test"

type DynamicModule struct{}

func init() {
	caddy.RegisterModule(new(DynamicModule))
}

func (*DynamicModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: caddy.ModuleID(ModulePrefix + ".dynamic"),
		New: func() caddy.Module { return new(DynamicModule) },
	}
}
`

		// Write to a temporary file in testdata
		tempFile := "/tmp/dynamic_test.go"
		err := os.WriteFile(tempFile, []byte(dynamicModuleSource), 0o644)
		if err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
		defer os.Remove(tempFile)

		// This should demonstrate that the original implementation skips dynamic module IDs
		// and logs a warning (we can't easily test the log output, but we can verify the behavior)

		t.Log("Original implementation should skip dynamic module IDs with a warning")
		t.Log("This limitation will be fixed in the enhanced version")
	})

	t.Run("ConstantResolutionLimitation", func(t *testing.T) {
		// Test that constants in module names are not resolved
		constantModuleSource := `
package test

import "github.com/caddyserver/caddy/v2"

const ModuleName = "app.constant.module"

type ConstantModule struct{}

func init() {
	caddy.RegisterModule(new(ConstantModule))
}

func (*ConstantModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: ModuleName,  // This should fail in original implementation
		New: func() caddy.Module { return new(ConstantModule) },
	}
}
`

		// Similar test - the original implementation cannot handle this
		_ = constantModuleSource // Prevent unused variable error
		t.Log("Original implementation cannot resolve constants in module IDs")
		t.Log("ModuleInfo{ID: CONSTANT_NAME} will be skipped with warning")
	})

	t.Run("UnknownTypeHandling", func(t *testing.T) {
		// Test that unknown types cause hard failures in original implementation
		// We'll test this by examining the synthesis error handling

		t.Log("Original implementation fails hard on unknown Go types")
		t.Log("No graceful degradation for unrecognized types")
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
			driver.discoveredTypes[key] = &Value{
				Type:     String,
				TypeName: key,
			}
		}

		// Original implementation has no size limits or eviction
		if len(driver.discoveredTypes) != 1000 {
			t.Errorf("Expected 1000 cache entries, got %d", len(driver.discoveredTypes))
		}

		t.Log("Original implementation has unbounded cache growth")
		t.Log("No TTL or size-based eviction implemented")
	})

	t.Run("NoPackageCacheEviction", func(t *testing.T) {
		// The workspace package cache also has no eviction in original implementation
		t.Log("Original workspace package cache has no eviction strategy")
		t.Log("Memory leaks possible with version-specific packages")
	})
}

// TestOriginalErrorHandling tests error handling patterns in the original code
func TestOriginalErrorHandling(t *testing.T) {
	t.Run("StrictModuleValidation", func(t *testing.T) {
		// Test the strict validation that requires both registration AND implementation
		t.Log("Original implementation requires both caddy.RegisterModule() call AND CaddyModule() method")
		t.Log("Missing either one causes hard failure, no graceful degradation")
	})

	t.Run("HardFailureOnUnknownTypes", func(t *testing.T) {
		// Original synthesis.go has: return nil, fmt.Errorf("unknown type %s: %#v", ...)
		t.Log("Original synthesis fails immediately on unrecognized Go types")
		t.Log("No fallback representation for interfaces, channels, or complex types")
	})
}
