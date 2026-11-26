package modshard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Two concurrent module compiles run on two threads of one worker JVM, not on two JVMs (#1656).
func TestCheckIsolatedRunsConcurrentModulesOnThreadsOfOneJVM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake worker is a shell script")
	}
	t.Setenv("KRIT_WORKER_AOT", "0")
	t.Setenv("KRIT_WORKER_THREADS", "")
	model, root := fixtureModel(t)
	writeModel(t, root, model)
	main := filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")
	test := filepath.Join(root, "lib", "src", "test", "kotlin", "LTest.kt")
	request, _ := json.Marshal(map[string]any{"id": 2, "files": []map[string]string{{"path": main}, {"path": test}}, "rules": []string{"R"}})
	dir := t.TempDir()
	java := filepath.Join(dir, "java")
	// Each invocation appends its arguments to calls.txt, then answers both modules.
	check := `{"succeeded":1,"findings":[],"rules":["R"],"errorFiles":{}}`
	var script strings.Builder
	script.WriteString("#!/bin/sh\necho \"$*\" >> '" + filepath.Join(dir, "calls.txt") + "'\n")
	script.WriteString("while [ $# -gt 0 ]; do\n  if [ \"$1\" = --facts-dir ]; then out=$2; fi\n  shift\ndone\n")
	for _, id := range []string{":lib:release", ":lib:releaseUnitTest"} {
		name := strings.TrimSuffix(factsFileName(id), ".json")
		script.WriteString("printf '%s' '" + check + "' > \"$out/" + name + ".check.json\"\n")
		script.WriteString("printf '%s' '{\"files\":{}}' > \"$out/" + name + ".json\"\n")
	}
	if err := os.WriteFile(java, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	_, rechecked, err := CheckIsolated(context.Background(), Input{
		ScanPaths: []string{root}, JarPath: filepath.Join(root, "krit-fir.jar"), JavaPath: java, Workers: 2,
		CacheDir: filepath.Join(t.TempDir(), "cache"), CheckRequest: request,
	}, []string{test})
	if err != nil || len(rechecked) != 2 {
		t.Fatalf("rechecked %v, err %v", rechecked, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "calls.txt"))
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(calls) != 1 || !strings.Contains(calls[0], "--module-threads 2") {
		t.Fatalf("want one JVM with --module-threads 2, got %d: %q", len(calls), calls)
	}
}
