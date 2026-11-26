package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The opt-in usage rules build one model from every source file: the marker, the marked API and the use live in
// different files, two reference hops apart. A warm run after editing only a use site, or only the marker's level,
// must report what a run without any findings cache reports (#1817).
func TestUnsafeOptInUsageWarmRunMatchesFreshRun(t *testing.T) {
	project := t.TempDir()
	files := map[string]string{
		"src/main/java/com/ex/api/ExperimentalJava.java": "package com.ex.api;\n\nimport androidx.annotation.RequiresOptIn;\n\n" +
			"@RequiresOptIn\npublic @interface ExperimentalJava {}\n",
		"src/main/kotlin/com/ex/api/Api.kt": "package com.ex.api\n\n@ExperimentalJava\nfun marked(): Int = 1\n",
		"src/main/java/com/ex/app/JavaUse.java": "package com.ex.app;\n\nimport com.ex.api.ApiKt;\n\nclass JavaUse {\n" +
			"    int use() {\n        return ApiKt.marked() + ApiKt.marked() + ApiKt.marked();\n    }\n}\n",
		"src/main/kotlin/com/ex/app/Use.kt": "package com.ex.app\n\nimport com.ex.api.marked\n\nfun use(): Int = marked() + marked()\n",
	}
	// Unrelated files keep the edit's affected set small next to the project, so the warm run replays it instead of
	// falling back to a full dispatch.
	for i := 0; i < 16; i++ {
		files[fmt.Sprintf("src/main/kotlin/com/ex/filler/Filler%d.kt", i)] = fmt.Sprintf("package com.ex.filler\n\nfun filler%d(): Int = %d\n", i, i)
	}
	for rel, src := range files {
		write(t, filepath.Join(project, filepath.FromSlash(rel)), src)
	}
	edit := func(rel, old, new string) {
		t.Helper()
		path := filepath.Join(project, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), old) {
			t.Fatalf("%s does not contain %q", rel, old)
		}
		write(t, path, strings.Replace(string(data), old, new, 1))
	}
	dispatchPath := regexp.MustCompile(`"path":\s*"([A-Za-z]+)"`)
	// counts returns the UnsafeOptInUsageError and UnsafeOptInUsageWarning findings, and the dispatch path.
	counts := func(args ...string) (errors, warnings int, path string) {
		t.Helper()
		base := []string{"--no-daemon", "--no-fir", "--no-type-oracle", "-f", "json", "--perf"}
		stdout, stderr, _ := runKrit(t, append(append(base, args...), project)...)
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
		}
		for _, f := range report.Findings {
			switch f.Rule {
			case "UnsafeOptInUsageError":
				errors++
			case "UnsafeOptInUsageWarning":
				warnings++
			}
		}
		if m := dispatchPath.FindStringSubmatch(stdout); m != nil {
			path = m[1]
		}
		return errors, warnings, path
	}
	// --no-cache does not yet bypass the cross-file findings cache (#1818), so the fresh run turns it off too.
	fresh := []string{"--no-cache", "--no-cross-file-cache"}

	if e, w, _ := counts(); e != 5 || w != 0 {
		t.Fatalf("cold run: %d errors, %d warnings, want 5 and 0", e, w)
	}

	edit("src/main/java/com/ex/app/JavaUse.java", "return ApiKt.marked() +", "return ApiKt.marked() + ApiKt.marked() +")
	e, w, path := counts()
	fe, fw, _ := counts(fresh...)
	t.Logf("after the use-site edit the warm run took dispatch path %q", path)
	if e != fe || w != fw || fe != 6 || fw != 0 {
		t.Fatalf("after the use-site edit: warm %d errors, %d warnings; fresh %d, %d; want 6 and 0", e, w, fe, fw)
	}

	edit("src/main/java/com/ex/api/ExperimentalJava.java", "@RequiresOptIn\n",
		"@RequiresOptIn(level = RequiresOptIn.Level.WARNING)\n")
	e, w, path = counts()
	fe, fw, _ = counts(fresh...)
	t.Logf("after the marker edit the warm run took dispatch path %q", path)
	if e != fe || w != fw || fe != 0 || fw != 6 {
		t.Fatalf("after the marker edit: warm %d errors, %d warnings; fresh %d, %d; want 0 and 6", e, w, fe, fw)
	}

	// A later warm run with no edit must not replay a stale result either.
	if e, w, _ := counts(); e != 0 || w != 6 {
		t.Fatalf("no-edit warm run: %d errors, %d warnings, want 0 and 6", e, w)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
