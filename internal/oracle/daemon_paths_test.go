package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cases := map[string]string{
		"":                           "",
		"src/main/kotlin":            filepath.Join(dir, "src", "main", "kotlin"),
		"./a/../b.kt":                filepath.Join(dir, "b.kt"),
		"/already/abs":               "/already/abs",
		"/unclean//abs/../spelling/": "/unclean//abs/../spelling/",
	}
	for in, want := range cases {
		if got := AbsolutePath(in); got != want {
			t.Errorf("AbsolutePath(%q) = %q, want %q", in, got, want)
		}
	}
	if AbsolutePaths(nil) != nil {
		t.Error("AbsolutePaths(nil) should be nil")
	}
}

func TestAbsoluteRequestPathsRestoresCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	abs, spelling := AbsoluteRequestPaths([]string{"a.kt", "/x/b.kt"})
	if want := []string{filepath.Join(dir, "a.kt"), "/x/b.kt"}; !reflect.DeepEqual(abs, want) {
		t.Fatalf("abs = %v, want %v", abs, want)
	}
	if got := spelling.Caller(abs[0]); got != "a.kt" {
		t.Errorf("Caller(%q) = %q, want a.kt", abs[0], got)
	}
	if got := spelling.Caller("/x/b.kt"); got != "/x/b.kt" {
		t.Errorf("Caller(/x/b.kt) = %q", got)
	}
	m := CallerKeys(spelling, map[string]int{abs[0]: 1, "/x/b.kt": 2, "/other.kt": 3})
	if want := map[string]int{"a.kt": 1, "/x/b.kt": 2, "/other.kt": 3}; !reflect.DeepEqual(m, want) {
		t.Errorf("CallerKeys = %v, want %v", m, want)
	}
}

// Two projects scanned as `krit .` both report the relative source root
// "src/main/kotlin"; the shared registry must not give them one daemon.
func TestDaemonRegistryKeyRelativeSourceDirsNameTheirProject(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rel := []string{filepath.Join("src", "main", "kotlin")}
	relCP := []string{filepath.Join("libs", "dep.jar")}

	t.Chdir(a)
	keyA := daemonRegistryKey(testJarPath, rel, relCP...)
	legacyA := daemonLegacyKey(testJarPath, rel, relCP...)
	t.Chdir(b)
	keyB := daemonRegistryKey(testJarPath, rel, relCP...)

	if keyA == keyB {
		t.Fatalf("relative sourceDirs of two projects share daemon key %q", keyA)
	}
	if legacyA == daemonLegacyKey(testJarPath, rel, relCP...) {
		t.Fatalf("relative sourceDirs of two projects share legacy key %q", legacyA)
	}
	absB := daemonRegistryKey(testJarPath, []string{filepath.Join(b, "src", "main", "kotlin")}, filepath.Join(b, "libs", "dep.jar"))
	if keyB != absB {
		t.Fatalf("relative spelling key %q != absolute spelling key %q", keyB, absB)
	}
}

func TestAppendDaemonJarArgsAbsolutizes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got := appendDaemonJarArgs(nil, "krit-types.jar", []string{"src/a", "/abs/b"}, []string{"lib.jar", "/abs/c.jar"}, "--daemon")
	want := []string{
		"-jar", filepath.Join(dir, "krit-types.jar"), "--daemon",
		"--sources", filepath.Join(dir, "src", "a") + ",/abs/b",
		"--classpath", filepath.Join(dir, "lib.jar") + string(os.PathListSeparator) + "/abs/c.jar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v\nwant   %v", got, want)
	}
}

// pipeDaemon connects a Daemon to handle, which answers each request line.
func pipeDaemon(t *testing.T, handle func(req daemonRequest) string) *Daemon {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	go func() {
		sc := bufio.NewScanner(server)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			var req daemonRequest
			if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
				return
			}
			_, _ = server.Write([]byte(handle(req) + "\n"))
		}
	}()
	reader := bufio.NewScanner(client)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	return &Daemon{stdin: client, stdout: reader, conn: client, nextID: 1, started: true, shared: true}
}

func requestFiles(req daemonRequest) []string {
	raw, _ := req.Params["files"].([]interface{})
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func TestAnalyzeWithDepsSendsAbsoluteFilesAndKeepsCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rel := filepath.Join("src", "A.kt")
	abs := filepath.Join(dir, rel)
	var sent []string
	d := pipeDaemon(t, func(req daemonRequest) string {
		sent = requestFiles(req)
		errs, _ := json.Marshal(map[string]string{sent[0]: "File not found in source module"})
		return fmt.Sprintf(`{"id":%d,"result":{"version":1,"files":{},"dependencies":{}},"cacheDeps":{},"errors":%s}`, req.ID, errs)
	})
	_, deps, err := d.AnalyzeWithDeps([]string{rel})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{abs}; !reflect.DeepEqual(sent, want) {
		t.Errorf("request files = %v, want %v", sent, want)
	}
	if _, ok := deps.Crashed[rel]; !ok || len(deps.Crashed) != 1 {
		t.Errorf("crashed = %v, want key %q", deps.Crashed, rel)
	}
}

func TestResolveExpressionTypesSendsAbsoluteFilesAndKeepsCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rel := filepath.Join("src", "A.kt")
	abs := filepath.Join(dir, rel)
	var sent map[string]interface{}
	d := pipeDaemon(t, func(req daemonRequest) string {
		sent, _ = req.Params["expressionPositions"].(map[string]interface{})
		types, _ := json.Marshal(map[string]map[string]resolvedExpressionFact{
			abs: {"1:2": {Name: "String", FQN: "kotlin.String"}},
		})
		return fmt.Sprintf(`{"id":%d,"result":{"types":%s}}`, req.ID, types)
	})
	out, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{rel: {{Line: 1, Col: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sent[abs]; !ok || len(sent) != 1 {
		t.Errorf("request positions keyed %v, want %q", sent, abs)
	}
	if got := out[rel][ExpressionPosition{Line: 1, Col: 2}]; got == nil || got.FQN != "kotlin.String" {
		t.Errorf("result for %q = %v; all: %v", rel, got, out)
	}
}
