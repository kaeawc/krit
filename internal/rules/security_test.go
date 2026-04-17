package rules_test

import (
	"strings"
	"testing"
)

func TestContentProviderQueryWithSelectionInterpolation_Positive(t *testing.T) {
	findings := runRuleByName(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

class UserLookup {
    fun load(resolver: Any, uri: Any, name: String) {
        resolver.query(uri, null, "name = '$name'", null, null)
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "selectionArgs") {
		t.Fatalf("expected selectionArgs guidance, got %q", findings[0].Message)
	}
}

func TestContentProviderQueryWithSelectionInterpolation_Negative(t *testing.T) {
	findings := runRuleByName(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

class UserLookup {
    fun load(resolver: Any, uri: Any, name: String) {
        resolver.query(uri, null, "name = ?", arrayOf(name), null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestContentProviderQueryWithSelectionInterpolation_IgnoresNonResolverQueries(t *testing.T) {
	findings := runRuleByName(t, "ContentProviderQueryWithSelectionInterpolation", `
package test

class DatabaseLookup {
    fun load(db: Any, tableName: String, name: String) {
        db.query(tableName, null, "name = '$name'", null, null)
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
