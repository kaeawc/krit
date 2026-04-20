package cache

import "github.com/kaeawc/krit/internal/scanner"

// UpdateEntry is a test-only slice-taking wrapper over UpdateEntryColumns.
// Production callers always hold columnar findings (main.go, LSP, MCP);
// this helper exists so cache_test.go can seed cache rows from
// []scanner.Finding fixtures.
func (c *Cache) UpdateEntry(path string, findings []scanner.Finding) {
	c.updateEntry(path, scanner.CollectFindings(findings))
}
