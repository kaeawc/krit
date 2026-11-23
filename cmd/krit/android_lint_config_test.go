package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// krit's port of an Android Lint issue stays quiet in a project whose
// lint.xml ignores that issue, as element-x-android and iosched ignore
// MissingTranslation.
func TestScanHonorsProjectLintXML(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"app/src/main/AndroidManifest.xml":        `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example"/>`,
		"app/src/main/res/values/strings.xml":     `<resources><string name="hello">Hello</string><string name="bye">Bye</string></resources>`,
		"app/src/main/res/values-fr/strings.xml":  `<resources><string name="hello">Bonjour</string></resources>`,
		"app/src/main/kotlin/com/example/Main.kt": "package com.example\n\nfun main() = Unit\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	missingTranslations := func() int {
		t.Helper()
		stdout, stderr, code := runKrit(t, "--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-q", dir)
		if code > 1 {
			t.Fatalf("krit exited %d: %s", code, stderr)
		}
		var report struct {
			Findings []scopeFinding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "MissingTranslation" {
				n++
			}
		}
		return n
	}
	if n := missingTranslations(); n != 1 {
		t.Fatalf("MissingTranslation findings without lint.xml = %d, want 1", n)
	}
	if err := os.WriteFile(filepath.Join(dir, "app", "lint.xml"), []byte(`<lint><issue id="MissingTranslation" severity="ignore"/></lint>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := missingTranslations(); n != 0 {
		t.Errorf("MissingTranslation findings with lint.xml ignoring it = %d, want 0", n)
	}
}
