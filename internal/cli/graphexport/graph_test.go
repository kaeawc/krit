package graphexport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGraphJSON(t *testing.T) {
	root := writeGraphProject(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format=json", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	var payload graphPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if payload.Scope != "module" {
		t.Fatalf("scope=%q, want module", payload.Scope)
	}
	if !hasEdge(payload.Edges, ":app", ":core", "implementation") {
		t.Fatalf("missing :app -> :core edge: %+v", payload.Edges)
	}
	if !hasEdge(payload.Edges, ":core", ":common", "api") {
		t.Fatalf("missing :core -> :common edge: %+v", payload.Edges)
	}
}

func TestRunGraphDOTAndMermaid(t *testing.T) {
	root := writeGraphProject(t)
	for _, tc := range []struct {
		format string
		want   string
	}{
		{format: "dot", want: `":app" -> ":core" [label="implementation"]`},
		{format: "mermaid", want: `-->|implementation|`},
	} {
		t.Run(tc.format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"--format=" + tc.format, root}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run code=%d stderr=%s", code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, tc.want) {
				t.Fatalf("%s output missing %q:\n%s", tc.format, tc.want, out)
			}
		})
	}
}

func TestRunPackageGraphJSON(t *testing.T) {
	root := writeGraphProject(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--scope=package", "--format=json", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	var payload graphPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if payload.Scope != "package" {
		t.Fatalf("scope=%q, want package", payload.Scope)
	}
	if !hasEdge(payload.Edges, "com.example.app", "com.example.core", "") {
		t.Fatalf("missing app package edge: %+v", payload.Edges)
	}
}

func TestRunGraphNoSettings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{t.TempDir()}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero without settings, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no settings.gradle(.kts) found") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func hasEdge(edges []graphEdge, from, to, label string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && (label == "" || edge.Label == label) {
			return true
		}
	}
	return false
}

func writeGraphProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeGraphFile(t, root, "settings.gradle.kts", `include(":app", ":core", ":common")
`)
	writeGraphFile(t, root, "app/build.gradle.kts", `plugins { kotlin("jvm") }
dependencies {
    implementation(project(":core"))
}
`)
	writeGraphFile(t, root, "core/build.gradle.kts", `plugins { kotlin("jvm") }
dependencies {
    api(project(":common"))
}
`)
	writeGraphFile(t, root, "common/build.gradle.kts", `plugins { kotlin("jvm") }
`)
	writeGraphFile(t, root, "app/src/main/kotlin/com/example/app/App.kt", `package com.example.app

import com.example.core.CoreThing

class App(val core: CoreThing)
`)
	writeGraphFile(t, root, "core/src/main/kotlin/com/example/core/CoreThing.kt", `package com.example.core

import com.example.common.CommonThing

class CoreThing(val common: CommonThing)
`)
	writeGraphFile(t, root, "common/src/main/kotlin/com/example/common/CommonThing.kt", `package com.example.common

class CommonThing
`)
	return root
}

func writeGraphFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
