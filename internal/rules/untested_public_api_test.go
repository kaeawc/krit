package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestUntestedPublicApi(t *testing.T) {
	t.Run("flags public production declarations without test references", func(t *testing.T) {
		findings := runUntestedPublicAPIFixture(t, map[string]string{
			"src/main/kotlin/com/example/UserRepository.kt": `
package com.example

class UserRepository {
    fun get(id: Long): String = id.toString()
}

fun orphanUtility(): String = "unused"
`,
			"src/test/kotlin/com/example/UserRepositoryTest.kt": `
package com.example

class UserRepositoryTest {
    fun unrelated() = "test"
}
`,
		})
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
		got := untestedPublicAPIFindingMessages(findings)
		if !strings.Contains(got, "UserRepository") || !strings.Contains(got, "orphanUtility") {
			t.Fatalf("expected class and top-level function findings, got %s", got)
		}
	})

	t.Run("skips referenced internal annotated and test declarations", func(t *testing.T) {
		findings := runUntestedPublicAPIFixture(t, map[string]string{
			"src/main/kotlin/com/example/UserRepository.kt": `
package com.example

import androidx.annotation.VisibleForTesting

class UserRepository {
    fun get(id: Long): String = id.toString()
}

fun coveredUtility(): String = "covered"

internal class InternalOnly

@VisibleForTesting
class TestHook
`,
			"src/test/kotlin/com/example/UserRepositoryTest.kt": `
package com.example

class UserRepositoryTest {
    fun coversRepository() {
        UserRepository()
        coveredUtility()
    }
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func runUntestedPublicAPIFixture(t *testing.T, files map[string]string) []scanner.Finding {
	t.Helper()
	root := t.TempDir()
	var parsed []*scanner.File
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
	}
	index := scanner.BuildIndex(parsed, 1)
	r := &UntestedPublicAPIRule{
		BaseRule: BaseRule{RuleName: "UntestedPublicApi", RuleSetName: testingQualityRuleSet, Sev: "info"},
	}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		CodeIndex:   index,
		ParsedFiles: parsed,
		Collector:   collector,
		Rule:        &api.Rule{ID: r.RuleName, Category: r.RuleSetName, Sev: api.Severity(r.Sev), Confidence: r.Confidence()},
	}
	r.check(ctx)
	return collector.Columns().Findings()
}

func untestedPublicAPIFindingMessages(findings []scanner.Finding) string {
	var b strings.Builder
	for _, finding := range findings {
		b.WriteString(finding.Message)
		b.WriteByte('\n')
	}
	return b.String()
}
