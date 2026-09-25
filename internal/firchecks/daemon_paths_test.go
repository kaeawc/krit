package firchecks

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// twoProjects returns two project roots that each hold src/main/kotlin, the
// source root `krit .` reports as the relative "src/main/kotlin" in both.
func twoProjects(t *testing.T) (a, b string) {
	t.Helper()
	root := t.TempDir()
	a, b = filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(filepath.Join(p, "src", "main", "kotlin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return a, b
}

func TestFirRegistryKeyRelativeSourceDirsNameTheirProject(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, b := twoProjects(t)
	rel := []string{filepath.Join("src", "main", "kotlin")}
	relCP := []string{filepath.Join("libs", "dep.jar")}

	t.Chdir(a)
	keyA := firRegistryKeyFor(firCheckRole, jar, rel, relCP)
	t.Chdir(b)
	keyB := firRegistryKeyFor(firCheckRole, jar, rel, relCP)

	if keyA == keyB {
		t.Fatalf("relative sourceDirs of two projects share daemon key %q", keyA)
	}
	absB := firRegistryKeyFor(firCheckRole, jar,
		[]string{filepath.Join(b, "src", "main", "kotlin")}, []string{filepath.Join(b, "libs", "dep.jar")})
	if keyB != absB {
		t.Fatalf("relative spelling key %q != absolute spelling key %q", keyB, absB)
	}
}

// A daemon started for project A (scanned as `krit .`) must not answer
// project B's `krit .` scan: its relative source roots name A's tree.
func TestConnectOrStartFirCheckDaemonDoesNotReuseAnotherProjectsDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, b := twoProjects(t)
	rel := []string{filepath.Join("src", "main", "kotlin")}

	// Register a live, responsive daemon for project A.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					_, _ = conn.Write([]byte("{}\n"))
				}
			}()
		}
	}()
	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = proc.Wait(); close(done) }()
	t.Cleanup(func() { _ = proc.Process.Kill(); <-done })

	t.Chdir(a)
	if err := writeFirPIDFile(proc.Process.Pid, listener.Addr().(*net.TCPAddr).Port, firRegistryKeyFor(firCheckRole, jar, rel, nil)); err != nil {
		t.Fatal(err)
	}
	d, err := connectOrStartFirCheckDaemon(jar, rel, nil, false)
	if err != nil {
		t.Fatalf("project A should reuse its own daemon: %v", err)
	}
	_ = d.Release()

	// From project B the same spelling must miss the registry; with no java
	// on PATH, starting B's own daemon then fails.
	t.Chdir(b)
	t.Setenv("PATH", t.TempDir())
	d, err = connectOrStartFirCheckDaemon(jar, rel, nil, false)
	if err == nil {
		_ = d.Release()
		t.Fatal("project B reused project A's daemon for the relative source root")
	}
	if !strings.Contains(err.Error(), "java") {
		t.Fatalf("expected a start failure, got %v", err)
	}
}

// pipeFirDaemon connects a FirDaemon to respond, which answers each request.
func pipeFirDaemon(t *testing.T, respond func(req firDaemonRequest) string) *FirDaemon {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	go func() {
		sc := bufio.NewScanner(server)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			var req firDaemonRequest
			if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
				return
			}
			_, _ = server.Write([]byte(respond(req) + "\n"))
		}
	}()
	reader := bufio.NewScanner(client)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	return &FirDaemon{conn: client, reader: reader, nextID: 1, started: true, shared: true}
}

// The oracle-backend RPCs send absolute paths and return facts keyed by the
// caller's spelling, walked (unrequested) files and closure edges included.
func TestFirOracleRPCsMapFactsToCallerSpelling(t *testing.T) {
	proj, _ := twoProjects(t)
	t.Chdir(proj)
	root := filepath.Join("src", "main", "kotlin")
	relA, relB := filepath.Join(root, "A.kt"), filepath.Join(root, "B.kt")
	absA, absB := filepath.Join(proj, relA), filepath.Join(proj, relB)
	var sent []firDaemonRequest
	d := pipeFirDaemon(t, func(req firDaemonRequest) string {
		sent = append(sent, req)
		files := `"files":{"` + absA + `":{"package":"p"},"` + absB + `":{"package":"p"}},"dependencies":{}`
		if req.Command == "analyzeWithDeps" {
			return `{"id":` + strconv.FormatInt(req.ID, 10) + `,"result":{` + files + `},` +
				`"cacheDeps":{"files":{"` + absB + `":{"depPaths":["` + absA + `"],"perFileDeps":{}}},"crashed":{"` + absB + `":"boom"}}}`
		}
		return `{"id":` + strconv.FormatInt(req.ID, 10) + `,"result":{` + files + `}}`
	})
	check := func(name string, files map[string]*oracle.File) {
		t.Helper()
		if files[relA] == nil || files[relB] == nil || len(files) != 2 {
			t.Errorf("%s: files = %v, want %q and %q", name, files, relA, relB)
		}
	}

	data, err := d.Analyze([]string{relA}, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	check("analyze", data.Files)
	data, err = d.AnalyzeAll([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	check("analyzeAll", data.Files)
	data, deps, err := d.AnalyzeWithDeps([]string{relA}, []string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	check("analyzeWithDeps", data.Files)
	if e := deps.Files[relB]; e == nil || !reflect.DeepEqual(e.DepPaths, []string{relA}) || len(deps.Files) != 1 {
		t.Errorf("cacheDeps.files = %v", deps.Files)
	}
	if _, ok := deps.Crashed[relB]; !ok || len(deps.Crashed) != 1 {
		t.Errorf("cacheDeps.crashed = %v", deps.Crashed)
	}
	for _, req := range sent {
		if !reflect.DeepEqual(req.SourceDirs, []string{filepath.Join(proj, root)}) {
			t.Errorf("%s sourceDirs = %v", req.Command, req.SourceDirs)
		}
		for _, f := range req.Files {
			if f.Path != absA {
				t.Errorf("%s file = %q, want %q", req.Command, f.Path, absA)
			}
		}
	}
}

// Check sends absolute paths (the daemon does not share the caller's
// working directory) and returns every path in the caller's spelling.
func TestCheckSendsAbsolutePathsAndKeepsCallerSpelling(t *testing.T) {
	proj, _ := twoProjects(t)
	t.Chdir(proj)
	relFile := filepath.Join("src", "main", "kotlin", "A.kt")
	absFile := filepath.Join(proj, relFile)
	absRoot := filepath.Join(proj, "src", "main", "kotlin")

	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	got := make(chan firDaemonRequest, 1)
	go func() {
		sc := bufio.NewScanner(server)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		if !sc.Scan() {
			return
		}
		var req firDaemonRequest
		_ = json.Unmarshal(sc.Bytes(), &req)
		got <- req
		path := ""
		if len(req.Files) > 0 {
			path = req.Files[0].Path
		}
		resp, _ := json.Marshal(CheckResponse{
			ID:         req.ID,
			Findings:   []FirFinding{{Path: path, Line: 1, Col: 1, Rule: "R"}},
			Crashed:    map[string]string{path: "crash"},
			ErrorFiles: map[string]string{path: "error"},
			RuleErrors: map[string]map[string]string{"R": {path: "threw"}},
		})
		_, _ = server.Write(append(resp, '\n'))
	}()
	reader := bufio.NewScanner(client)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	d := &FirDaemon{conn: client, reader: reader, nextID: 1, started: true, shared: true}

	resp, err := d.Check([]fileRef{{Path: relFile, ContentHash: "h"}},
		[]string{filepath.Join("src", "main", "kotlin")}, []string{"dep.jar"}, []string{"R"}, nil, []string{relFile})
	if err != nil {
		t.Fatal(err)
	}
	req := <-got
	if want := []fileRef{{Path: absFile, ContentHash: "h"}}; !reflect.DeepEqual(req.Files, want) {
		t.Errorf("request files = %v, want %v", req.Files, want)
	}
	if want := []string{absRoot}; !reflect.DeepEqual(req.SourceDirs, want) {
		t.Errorf("request sourceDirs = %v, want %v", req.SourceDirs, want)
	}
	if want := []string{filepath.Join(proj, "dep.jar")}; !reflect.DeepEqual(req.Classpath, want) {
		t.Errorf("request classpath = %v, want %v", req.Classpath, want)
	}
	if want := []string{absFile}; !reflect.DeepEqual(req.TestFiles, want) {
		t.Errorf("request testFiles = %v, want %v", req.TestFiles, want)
	}

	if len(resp.Findings) != 1 || resp.Findings[0].Path != relFile {
		t.Errorf("findings = %+v, want path %q", resp.Findings, relFile)
	}
	for name, m := range map[string]map[string]string{"crashed": resp.Crashed, "errorFiles": resp.ErrorFiles, "ruleErrors[R]": resp.RuleErrors["R"]} {
		if _, ok := m[relFile]; !ok || len(m) != 1 {
			t.Errorf("%s = %v, want key %q", name, m, relFile)
		}
	}
}
