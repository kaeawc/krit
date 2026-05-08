package cacheutil

// ClearRegistryForTesting resets the global registry to empty.
// Only for use in tests.
func ClearRegistryForTesting() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
}
