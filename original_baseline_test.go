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
	"testing"
)

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
