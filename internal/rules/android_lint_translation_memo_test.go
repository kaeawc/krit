package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestTranslationFindingsForSharesAnalysisPerIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "res")
	for _, d := range []string{"values", "values-fr"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, body string) {
		if err := os.WriteFile(filepath.Join(root, dir, "strings.xml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("values", `<resources><string name="a">A</string></resources>`)
	write("values-fr", `<resources></resources>`)
	idx := &android.ResourceIndex{}
	roots := []string{root}
	first := translationFindingsFor(idx, roots)
	if len(first) == 0 {
		t.Fatal("expected a missing translation finding")
	}
	write("values-fr", `<resources><string name="a">A</string></resources>`)
	if again := translationFindingsFor(idx, roots); len(again) != len(first) {
		t.Fatalf("same index should reuse analysis: %d vs %d", len(again), len(first))
	}
	if fresh := translationFindingsFor(&android.ResourceIndex{}, roots); len(fresh) != 0 {
		t.Fatalf("new index should re-analyze, got %d findings", len(fresh))
	}
}
