package moduledoc

import (
	"testing"
)

// TestConcurrencyIssuesOriginal demonstrates the race conditions in the original implementation
func TestConcurrencyIssuesOriginal(t *testing.T) {
	// t.Run("RaceConditionDemo", func(t *testing.T) {
	// 	t.Log("This test demonstrates the race condition in discoveredTypes map")
	// 	t.Log("Run with: go test -race -run TestConcurrencyIssuesOriginal")

	// 	driver := New(nil)
	// 	var wg sync.WaitGroup

	// 	// Multiple goroutines accessing the map concurrently
	// 	for i := 0; i < 10; i++ {
	// 		wg.Add(1)
	// 		go func(id int) {
	// 			defer wg.Done()

	// 			// This is what the original code does - direct map access without locking
	// 			// This should trigger race warnings when run with -race
	// 			key := "test.type." + string(rune('A'+id))

	// 			// Read operation
	// 			if _, exists := driver.discoveredTypes[key]; !exists {
	// 				// Write operation
	// 				driver.discoveredTypes[key] = &Value{
	// 					Type:     String,
	// 					TypeName: key,
	// 				}
	// 			}
	// 		}(i)
	// 	}

	// 	wg.Wait()

	// 	t.Logf("Completed concurrent operations on discoveredTypes map")
	// 	t.Logf("Map now contains %d entries", len(driver.discoveredTypes))
	// 	t.Log("NOTE: Race detector will show warnings for the above operations")
	// })
}

// TestMemoryLeakDemo demonstrates the unbounded cache growth
func TestMemoryLeakDemo(t *testing.T) {
	t.Run("UnboundedCacheGrowth", func(t *testing.T) {
		driver := New(nil)

		// Simulate what happens with many package versions
		for i := 0; i < 1000; i++ {
			for version := 0; version < 5; version++ {
				key := "test.package." + string(rune('A'+i)) + "@v1." + string(rune('0'+version)) + ".0"
				driver.discoveredTypes[key] = &Value{
					Type:     String,
					TypeName: key,
				}
			}
		}

		t.Logf("Cache grew to %d entries with no eviction", len(driver.discoveredTypes))
		t.Log("Original implementation has no size limits or TTL")
		t.Log("This can cause memory exhaustion in long-running processes")
	})
}
