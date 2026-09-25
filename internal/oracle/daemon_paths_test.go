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

func TestPathSpellingWithDirsMapsWalkedFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join("src", "main", "kotlin")
	nested := filepath.Join(root, "gen")
	abs, spelling := AbsoluteRequestPaths([]string{"./" + filepath.Join(root, "A.kt")})
	spelling = spelling.WithDirs([]string{root, nested, "/abs/root"})
	cases := map[string]string{
		// A request keeps its exact spelling, even an unclean one.
		abs[0]: "./" + filepath.Join(root, "A.kt"),
		// Walked files are re-spelled under the caller's root, the way
		// filepath.Walk spells them; the longest root wins.
		filepath.Join(dir, root, "p", "B.kt"): filepath.Join(root, "p", "B.kt"),
		filepath.Join(dir, nested, "C.kt"):    filepath.Join(nested, "C.kt"),
		filepath.Join(dir, root):              filepath.Join(dir, root),
		filepath.Join(dir, "src", "other.kt"): filepath.Join(dir, "src", "other.kt"),
		filepath.Join(dir, root+"x", "D.kt"):  filepath.Join(dir, root+"x", "D.kt"),
		filepath.Join("/abs", "root", "E.kt"): filepath.Join("/abs", "root", "E.kt"),
		filepath.Join("/elsewhere", "F.kt"):   filepath.Join("/elsewhere", "F.kt"),
	}
	for in, want := range cases {
		if got := spelling.Caller(in); got != want {
			t.Errorf("Caller(%q) = %q, want %q", in, got, want)
		}
	}
}

// A request's own spelling wins over a source-root re-spelling, also when
// the request was already absolute (serve/LSP callers send absolute files
// on handles started with relative roots).
func TestPathSpellingAbsoluteRequestKeepsItsSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join("src", "main", "kotlin")
	absA := filepath.Join(dir, root, "A.kt")
	absB := filepath.Join(dir, root, "B.kt")
	abs, spelling := AbsoluteRequestPaths([]string{absA})
	spelling = spelling.WithDirs([]string{root})
	if abs[0] != absA {
		t.Fatalf("abs = %v", abs)
	}
	if got := spelling.Caller(absA); got != absA {
		t.Errorf("Caller(requested %q) = %q, want it unchanged", absA, got)
	}
	// An unrequested file under the root still takes the root's spelling.
	if got, want := spelling.Caller(absB), filepath.Join(root, "B.kt"); got != want {
		t.Errorf("Caller(walked %q) = %q, want %q", absB, got, want)
	}
	m := CallerKeys(spelling, map[string]int{absA: 1, absB: 2})
	if want := map[string]int{absA: 1, filepath.Join(root, "B.kt"): 2}; !reflect.DeepEqual(m, want) {
		t.Errorf("CallerKeys = %v, want %v", m, want)
	}
	// With no rewrite and no roots, responses pass through untouched.
	_, plain := AbsoluteRequestPaths([]string{absA})
	if filepath.Separator == '/' && !plain.identity() {
		t.Error("absolute-only request with no roots should be the identity mapping")
	}
}

// krit-types' virtual file system reports forward-slash paths on Windows;
// they must still match the native request and root spellings.
func TestPathSpellingMatchesForwardSlashEchoes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join("src", "main", "kotlin")
	rel := filepath.Join(root, "A.kt")
	abs, spelling := AbsoluteRequestPaths([]string{rel})
	spelling = spelling.WithDirs([]string{root})
	if got := spelling.Caller(filepath.ToSlash(abs[0])); got != rel {
		t.Errorf("Caller(%q) = %q, want %q", filepath.ToSlash(abs[0]), got, rel)
	}
	walked := filepath.ToSlash(filepath.Join(dir, root, "p", "B.kt"))
	if got, want := spelling.Caller(walked), filepath.Join(root, "p", "B.kt"); got != want {
		t.Errorf("Caller(%q) = %q, want %q", walked, got, want)
	}
}

// A relative request must come back keyed the way it was asked, including
// files the daemon reports under the source root without being asked, the
// closure's edges, and crash markers.
func TestAnalyzeWithDepsMapsDataAndCacheDepsToCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join("src", "main", "kotlin")
	relA, relB := filepath.Join(root, "A.kt"), filepath.Join(root, "B.kt")
	absA, absB := filepath.Join(dir, relA), filepath.Join(dir, relB)
	d := pipeDaemon(t, func(req daemonRequest) string {
		return fmt.Sprintf(`{"id":%d,"result":{"version":1,"files":{%q:{"package":"p"},%q:{"package":"p"}},"dependencies":{}},`+
			`"cacheDeps":{"version":1,"files":{%q:{"depPaths":[%q],"propagatingDepPaths":[%q],"perFileDeps":{}}},"crashed":{%q:"boom"}}}`,
			req.ID, absA, absB, absB, absA, absA, absB)
	})
	d.sourceDirs = []string{root}
	data, deps, err := d.AnalyzeWithDeps([]string{relA})
	if err != nil {
		t.Fatal(err)
	}
	if data.Files[relA] == nil || data.Files[relB] == nil || len(data.Files) != 2 {
		t.Errorf("Files keys = %v, want %q and %q", mapKeys(data.Files), relA, relB)
	}
	entry := deps.Files[relB]
	if entry == nil || len(deps.Files) != 1 {
		t.Fatalf("CacheDeps.Files keys = %v, want %q", mapKeys(deps.Files), relB)
	}
	if !reflect.DeepEqual(entry.DepPaths, []string{relA}) || !reflect.DeepEqual(entry.PropagatingDepPaths, []string{relA}) {
		t.Errorf("edges = %v / %v, want [%q]", entry.DepPaths, entry.PropagatingDepPaths, relA)
	}
	if _, ok := deps.Crashed[relB]; !ok || len(deps.Crashed) != 1 {
		t.Errorf("Crashed = %v, want key %q", deps.Crashed, relB)
	}
}

func TestAnalyzeAndAnalyzeAllMapFilesToCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join("src", "main", "kotlin")
	rel := filepath.Join(root, "A.kt")
	d := pipeDaemon(t, func(req daemonRequest) string {
		return fmt.Sprintf(`{"id":%d,"result":{"version":1,"files":{%q:{"package":"p"}},"dependencies":{}}}`,
			req.ID, filepath.Join(dir, rel))
	})
	d.sourceDirs = []string{root}
	for name, call := range map[string]func() (*Data, error){
		"analyze":      func() (*Data, error) { return d.Analyze([]string{rel}) },
		"analyzeAll":   d.AnalyzeAll,
		"analyzeFiles": func() (*Data, error) { return d.AnalyzeFilesWithCallFilter([]string{rel}, nil) },
	} {
		data, err := call()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if data.Files[rel] == nil || len(data.Files) != 1 {
			t.Errorf("%s: Files keys = %v, want %q", name, mapKeys(data.Files), rel)
		}
	}
}

func TestAnalyzePluginFileKeepsCallerSpelling(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rel := filepath.Join("src", "A.kt")
	abs := filepath.Join(dir, rel)
	var sentPath string
	d := pipeDaemon(t, func(req daemonRequest) string {
		sentPath, _ = req.Params["path"].(string)
		return fmt.Sprintf(`{"id":%d,"result":{"findings":[{"file":%q,"line":1,"column":1,"ruleId":"R"}],"errors":{%q:"x"}}}`,
			req.ID, abs, abs)
	})
	out, err := d.AnalyzePluginFile(nil, rel, []byte("x"), []string{"R"}, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sentPath != abs {
		t.Errorf("request path = %q, want %q", sentPath, abs)
	}
	if len(out.Findings) != 1 || out.Findings[0].File != rel {
		t.Errorf("findings = %+v, want file %q", out.Findings, rel)
	}
	if _, ok := out.Errors[rel]; !ok || len(out.Errors) != 1 {
		t.Errorf("errors = %v, want key %q", out.Errors, rel)
	}
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
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
