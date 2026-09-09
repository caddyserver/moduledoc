package moduledoc

import (
	"testing"
)

// TestMemoryLeakDemo demonstrates the unbounded cache growth
func TestMemoryLeakDemo(t *testing.T) {
	t.Run("UnboundedCacheGrowth", func(t *testing.T) {
		driver := New(nil)

		// Simulate what happens with many package versions
		for i := 0; i < 1000; i++ {
			for version := 0; version < 5; version++ {
				key := "test.package." + string(rune('A'+i)) + "@v1." + string(rune('0'+version)) + ".0"
				driver.setDiscoveredType(key, &Value{
					Type:     String,
					TypeName: key,
				})
			}
		}

		// no size limit, TTL, or eviction: every entry is retained
		if len(driver.discoveredTypes) != 5000 {
			t.Errorf("expected 5000 retained entries, got %d", len(driver.discoveredTypes))
		}
	})
}
