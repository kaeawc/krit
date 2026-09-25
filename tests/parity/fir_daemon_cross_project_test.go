package parity_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

const crossProjectStub = `package kotlinx.coroutines

object Dispatchers {
    val IO: Any = Any()
}

suspend fun <T> withContext(context: Any, block: suspend () -> T): T = block()
`

// Two projects scanned the way `krit --fir .` scans them, from inside each
// project, both report the relative source root "src/main/kotlin". The FIR
// checker daemon they share through the user-level registry must still check
// each project against its own sources: the second project's hardcoded
// dispatcher is found, and none of its files is reported as outside the
// compilation.
func TestFirCheckDaemonRelativeScansOfTwoProjectsStaySeparate(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	// A private registry: the daemons this test starts are its own.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Cleanup(func() { killRegisteredDaemons(t, home) })

	base := t.TempDir()
	projects := map[string]string{
		// A: a hardcoded dispatcher Go's rule already reports.
		"a": "package app\n\nimport kotlinx.coroutines.Dispatchers\nimport kotlinx.coroutines.withContext\n\n" +
			"class Service {\n    suspend fun load(): Int = withContext(Dispatchers.IO) { 1 }\n}\n",
		// B: the same class behind an import alias, which only the compiler
		// resolves to kotlinx.coroutines.Dispatchers.IO.
		"b": "package app\n\nimport kotlinx.coroutines.Dispatchers as D\nimport kotlinx.coroutines.withContext\n\n" +
			"class Service {\n    suspend fun load(): Int = withContext(D.IO) { 2 }\n}\n",
	}
	for _, name := range []string{"a", "b"} {
		src := filepath.Join(base, name, "src", "main", "kotlin")
		for _, pkg := range []string{filepath.Join("kotlinx", "coroutines"), "app"} {
			if err := os.MkdirAll(filepath.Join(src, pkg), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, filepath.Join(src, "kotlinx", "coroutines", "Stub.kt"), crossProjectStub)
		writeFile(t, filepath.Join(src, "app", "Service.kt"), projects[name])
	}

	for _, name := range []string{"a", "b"} {
		dir := filepath.Join(base, name)
		t.Chdir(dir)
		sourceDirs := oracle.FindSourceDirs([]string{"."})
		if len(sourceDirs) != 1 || filepath.IsAbs(sourceDirs[0]) {
			t.Fatalf("project %s: sourceDirs = %v, want one relative root", name, sourceDirs)
		}
		rel, err := firchecks.CollectFirKtFiles([]string{"."})
		if err != nil {
			t.Fatal(err)
		}
		files := make([]string, len(rel))
		for i, p := range rel {
			files[i] = filepath.Join(dir, p)
		}
		res, err := firchecks.InvokeCached(jar, files, sourceDirs, nil, []string{"InjectDispatcher"}, nil, nil, "", true, false)
		if err != nil {
			t.Fatalf("project %s: %v", name, err)
		}
		if len(res.ErrorFiles) > 0 || len(res.Crashed) > 0 {
			t.Fatalf("project %s: errorFiles=%v crashed=%v", name, res.ErrorFiles, res.Crashed)
		}
		service := filepath.Join(dir, "src", "main", "kotlin", "app", "Service.kt")
		if len(res.Findings) != 1 || res.Findings[0].File != service || res.Findings[0].Rule != "InjectDispatcher" {
			t.Fatalf("project %s: findings = %+v, want one InjectDispatcher in %s", name, res.Findings, service)
		}
	}
}

// killRegisteredDaemons stops every daemon registered under home.
func killRegisteredDaemons(t *testing.T, home string) {
	t.Helper()
	pids, _ := filepath.Glob(filepath.Join(home, ".krit", "cache", "daemons", "*.pid"))
	for _, path := range pids {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 0 {
			continue
		}
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	}
}
