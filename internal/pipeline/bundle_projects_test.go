package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// #1635: the daemon's warm bundle-hit route formats the same per-project
// attribution as a cold run, and a model change is a different cached output.
func TestBundleHitOutputCarriesProjectAttribution(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	writeModel := func(name, project string) {
		t.Helper()
		dir := filepath.ToSlash(filepath.Join(root, strings.TrimPrefix(project, ":")))
		body, _ := json.Marshal(map[string]any{"schema": 2, "projects": []any{map[string]any{
			"path": project, "dir": dir,
			"sourceSets": []any{map[string]any{"name": "main", "sourceDirs": []string{dir + "/src/main/kotlin"}}},
		}}})
		if err := os.MkdirAll(filepath.Join(root, strings.TrimPrefix(project, ":")), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(modelDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modelDir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeModel("app.json", ":app")
	cached := scanner.CollectFindings([]scanner.Finding{{
		File: filepath.Join(root, "app", "src", "main", "kotlin", "A.kt"), Line: 1, Col: 1,
		RuleSet: "style", Rule: "UnusedVariable", Severity: "warning", Message: "m",
	}})
	serve := bundleHitServer(t, root)
	for _, run := range []struct {
		name    string
		columns *scanner.FindingColumns
	}{{"cold", &cached}, {"warm", nil}} {
		out, ok := serve(run.columns)
		if !ok {
			t.Fatalf("%s: not served", run.name)
		}
		if !strings.Contains(out, `"project":":app"`) || !strings.Contains(out, `"byProject":{":app":{"findings":1,"errors":0,"warnings":1}}`) {
			t.Fatalf("%s run lacks the project attribution:\n%s", run.name, out)
		}
	}

	// Another model is another cached output: the warm entry is not replayed for it.
	writeModel("lib.json", ":lib")
	if out, ok := serve(nil); ok {
		t.Fatalf("a changed model replayed the old cached output:\n%s", out)
	}
}

// bundleHitServer serves a JSON report of root through the daemon's bundle-hit
// output cache: columns stores an entry, nil replays one. The bool reports a serve.
func bundleHitServer(t *testing.T, root string) func(*scanner.FindingColumns) (string, bool) {
	fp := scanner.RunFingerprint{Version: "test", Rules: "rh", Config: "cfg"}
	args := ProjectArgs{Format: "json", JSONCompact: true, Paths: []string{root}, BasePath: root}
	w := NewWorkspaceState("")
	host := ProjectHostState{DaemonCaches: DaemonCaches{BundleOutput: w.BundleOutput, StoreBundleOutput: w.StoreBundleOutput}}
	return func(columns *scanner.FindingColumns) (string, bool) {
		t.Helper()
		var buf strings.Builder
		_, ok, err := serveBundleHitFromOutputCache(columns, fp, nil, nil, args, host, &buf,
			time.Time{}, nil, nil, nil, args.gradleProjects())
		if err != nil {
			t.Fatal(err)
		}
		return buf.String(), ok
	}
}

// The cached output's key follows what the model attributes, not only its bytes:
// gradlemodel.Load skips a project whose directory is missing, and a same-size
// rewrite within one timestamp tick is new content.
func TestBundleHitOutputKeyFollowsAcceptedProjects(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, ".krit", "gradle-model", "app.json")
	writeModel := func(project string) {
		t.Helper()
		dir := filepath.ToSlash(filepath.Join(root, "app"))
		body, _ := json.Marshal(map[string]any{"schema": 2, "projects": []any{map[string]any{
			"path": project, "dir": dir,
			"sourceSets": []any{map[string]any{"name": "main", "sourceDirs": []string{dir + "/src/main/kotlin"}}},
		}}})
		if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modelPath, body, 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(1_700_000_000, 0)
		if err := os.Chtimes(modelPath, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	cached := scanner.CollectFindings([]scanner.Finding{{
		File: filepath.Join(root, "app", "src", "main", "kotlin", "A.kt"), Line: 1, Col: 1,
		RuleSet: "style", Rule: "UnusedVariable", Severity: "warning", Message: "m",
	}})
	serve := bundleHitServer(t, root)

	// :app's directory is missing, so the model attributes nothing.
	writeModel(":app")
	if out, ok := serve(&cached); !ok || strings.Contains(out, `"project":":app"`) {
		t.Fatalf("served = %v; a project without its directory must not be attributed:\n%s", ok, out)
	}
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, ok := serve(nil); ok {
		t.Fatalf("creating :app's directory replayed the output that lacks it:\n%s", out)
	}
	if out, ok := serve(&cached); !ok || !strings.Contains(out, `"project":":app"`) {
		t.Fatalf("served = %v; want :app attributed once its directory exists:\n%s", ok, out)
	}

	// Same size, same mtime, another project path.
	writeModel(":apq")
	if out, ok := serve(nil); ok {
		t.Fatalf("a same-size model rewrite replayed the old output:\n%s", out)
	}
}
