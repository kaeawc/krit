package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppliedGradlePluginIDsIgnoresCommentsAndApplyFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build.gradle.kts")
	body := `plugins {
    id("real.plugin")
    // id("commented.plugin")
    id("root.only") apply false
}
apply(plugin = "legacy.plugin")
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := appliedGradlePluginIDs(path)
	for _, want := range []string{"real.plugin", "legacy.plugin"} {
		if !got[want] {
			t.Errorf("expected %q to be applied, got %v", want, got)
		}
	}
	for _, unwanted := range []string{"commented.plugin", "root.only"} {
		if got[unwanted] {
			t.Errorf("did not expect %q to be applied, got %v", unwanted, got)
		}
	}
}

func TestAnnotationTextMentionsBoundaries(t *testing.T) {
	cases := []struct {
		text, name string
		want       bool
	}{
		{"@Dao", "Dao", true},
		{"@DaoHelper", "Dao", false},
		{"@androidx.room.Dao", "Dao", true},
		{"@get:Dao", "Dao", true},
		{`@Suppress("@Dao")`, "Dao", false},
		{"@DependencyGraph.Factory", "DependencyGraph.Factory", true},
		{"@Query(\"x\") @Dao", "Dao", true},
		{"@DatabaseView(\"x\")", "Database", false},
	}
	for _, c := range cases {
		if got := annotationTextMentions(c.text, c.name); got != c.want {
			t.Errorf("annotationTextMentions(%q, %q) = %v, want %v", c.text, c.name, got, c.want)
		}
	}
}
