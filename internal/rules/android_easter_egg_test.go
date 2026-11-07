package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// escape is the unicode escape for a comment end, spelled out so this file stays free of it.
const easterEggEscape = `\u002a\u002f`

func runEasterEgg(t *testing.T, java string, xml map[string]string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var parsed []*scanner.File
	if java == "" {
		java = "package test;\nclass Empty {}\n" // the source set is found through its code
	}
	if java != "" {
		path := filepath.Join(dir, "src", "main", "java", "A.java")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(java), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseJavaFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
	}
	for name, body := range xml {
		path := filepath.Join(dir, "src", "main", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "EasterEgg" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("EasterEgg not registered")
	}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{Rule: rule, ParsedFiles: parsed, Collector: collector, DefaultConfidence: 0.9}
	rule.Check(ctx)
	return api.ContextFindings(ctx)
}

func easterEggLines(findings []scanner.Finding) []int {
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	return lines
}

func TestEasterEgg_FollowsLint(t *testing.T) {
	const message = "Code might be hidden here; found unicode escape sequence which is interpreted as comment end, compiled code follows"
	t.Run("Java line and block comments", func(t *testing.T) {
		findings := runEasterEgg(t, "package test;\npublic class A {\n    /* hidden "+easterEggEscape+" int x = 1; */\n    // line "+easterEggEscape+" int y = 2;\n"+
			"    // upper \\U002A\\U002F x\n    // half \\u002a/ x\n    // spaced \\u002a \\u002f\n    String s = \"*/\";\n}\n", map[string]string{})
		if got := easterEggLines(findings); len(got) != 3 || got[0] != 3 || got[1] != 4 || got[2] != 5 {
			t.Fatalf("expected findings on lines 3, 4 and 5, got %v", got)
		}
		if findings[0].Message != message {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
		for _, f := range findings {
			if f.Line == 3 && f.Col != 15 {
				t.Fatalf("expected the finding on the escape (column 15), got %d", f.Col)
			}
		}
	})
	t.Run("javadoc comments are not checked", func(t *testing.T) {
		findings := runEasterEgg(t, "package test;\npublic class A {\n    /** javadoc "+easterEggEscape+" */\n    int x;\n}\n", map[string]string{})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", easterEggLines(findings))
		}
	})
	t.Run("XML comments in resources and the manifest", func(t *testing.T) {
		findings := runEasterEgg(t, "", map[string]string{
			"AndroidManifest.xml": "<manifest>\n    <!-- manifest " + easterEggEscape + " -->\n</manifest>\n",
			"res/layout/main.xml": "<!-- before root " + easterEggEscape + " -->\n<LinearLayout>\n    <!-- multi\n      line " + easterEggEscape + "\n    -->\n</LinearLayout>\n",
			"res/values/more.xml": "<resources>\n    <string name=\"b\">b " + easterEggEscape + "</string>\n</resources>\n",
		})
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings (manifest, two layout comments), got %d", len(findings))
		}
		for _, f := range findings {
			switch filepath.Base(f.File) {
			case "AndroidManifest.xml":
				if f.Line != 2 {
					t.Errorf("manifest finding on line %d, want 2", f.Line)
				}
			case "main.xml":
				if f.Line != 1 && f.Line != 3 {
					t.Errorf("layout finding on line %d, want 1 or 3", f.Line)
				}
			default:
				t.Errorf("unexpected finding in %s: text outside a comment is not checked", f.File)
			}
		}
	})
}
