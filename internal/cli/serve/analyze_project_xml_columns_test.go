package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// The daemon answers many scans from one process, so the XML line cache that
// anchors line-only XML findings must follow an edit: re-indenting a string
// and adding a line above it moves UnusedResources from 2:5 to 3:3, as a
// fresh scan reports (#1852).
func TestAnalyzeProject_XMLFindingColumnsFollowEdits(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	root := state.root
	for name, content := range map[string]string{
		"settings.gradle.kts":                 "include(\":app\")\n",
		"app/build.gradle.kts":                "plugins { id(\"com.android.application\") }\n",
		"app/src/main/AndroidManifest.xml":    "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"p\"><application/></manifest>\n",
		"app/src/main/kotlin/p/F.kt":          "package p\n\nfun f() = 1\n",
		"app/src/main/res/values/strings.xml": "<resources>\n    <string name=\"unused_one\">One</string>\n</resources>\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		// Old enough that the line cache may keep the file between scans.
		old := time.Now().Add(-time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	strings := filepath.Join(root, "app/src/main/res/values/strings.xml")

	unused := func() (line, col int) {
		t.Helper()
		var r daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, daemon.AnalyzeProjectArgs{NoFir: true, EnableRules: "UnusedResources"}, &r); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Findings []struct {
				Rule   string `json:"rule"`
				File   string `json:"file"`
				Line   int    `json:"line"`
				Column int    `json:"column"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(r.Findings, &report); err != nil {
			t.Fatalf("findings JSON: %v", err)
		}
		for _, f := range report.Findings {
			if f.Rule == "UnusedResources" && filepath.Base(f.File) == "strings.xml" {
				return f.Line, f.Column
			}
		}
		t.Fatalf("no UnusedResources finding in strings.xml: %s", r.Findings)
		return 0, 0
	}
	if line, col := unused(); line != 2 || col != 5 {
		t.Fatalf("first scan: UnusedResources at %d:%d, want 2:5", line, col)
	}

	if err := os.WriteFile(strings, []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<resources>\n  <string name=\"unused_one\">One</string>\n</resources>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state.workspace.Touch(strings)
	state.workspace.BumpSourceMTimeVersion()
	if line, col := unused(); line != 3 || col != 3 {
		t.Fatalf("after re-indenting and adding a line: UnusedResources at %d:%d, want 3:3", line, col)
	}
}
