package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// #810: a daemon-delegated `krit --fir` scan compiles the module once, for
// the resident oracle daemon and the FIR checkers together, and reports what
// the in-process scan (which has always shared its compile, #739) reports.

type sharedScan struct {
	Findings   []string
	OutcomeLog []string
}

type sharedScanOutput struct {
	Findings []struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Rule    string `json:"rule"`
		Message string `json:"message"`
	} `json:"findings"`
	PerfTiming []sharedPerfEntry `json:"perfTiming"`
}

type sharedPerfEntry struct {
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
	Children   []sharedPerfEntry `json:"children"`
}

func (e sharedPerfEntry) collect(name string, out *[]string) {
	if e.Name == name {
		*out = append(*out, e.Attributes["outcome"])
	}
	for _, c := range e.Children {
		c.collect(name, out)
	}
}

func runSharedScan(t *testing.T, bin, jar, home, project string, delegated bool) sharedScan {
	t.Helper()
	args := []string{"--fir", "-f", "json", "--perf", "--enable-rules", "InjectDispatcher", "--no-cache"}
	if !delegated {
		args = append(args, "--no-daemon")
	}
	cmd := exec.Command(bin, append(args, project)...)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "HOME="+home, "KRIT_FIR_JAR="+jar)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
		t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	if delegated && !strings.Contains(stderr.String(), "using daemon") {
		t.Skipf("the delegated scan did not reach a daemon here:\n%s", stderr.String())
	}
	var out sharedScanOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("parse krit output: %v\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	var scan sharedScan
	for _, f := range out.Findings {
		rel, relErr := filepath.Rel(project, f.File)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			rel = f.File
		}
		scan.Findings = append(scan.Findings, fmt.Sprintf("%s:%d:%d %s %s", filepath.ToSlash(rel), f.Line, f.Column, f.Rule, f.Message))
	}
	sort.Strings(scan.Findings)
	for _, e := range out.PerfTiming {
		e.collect("firCheckBroker", &scan.OutcomeLog)
	}
	return scan
}

// stopDelegatedDaemon stops the krit daemon (and the JVMs it owns) that a
// delegated scan of project left running under home.
func stopDelegatedDaemon(t *testing.T, bin, home, project string) {
	t.Helper()
	cmd := exec.Command(bin, "daemon", "stop")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "HOME="+home)
	_ = cmd.Run()
	killRegisteredDaemons(t, home)
}

func TestDelegatedFirScanSharesTheCompileAndMatchesInProcess(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	bin := buildVerdictKrit(t, root)
	// A private HOME keeps the daemon registry, the AOT caches and the
	// delegated daemon this test starts its own.
	home := t.TempDir()

	inProcess := runSharedScan(t, bin, jar, home, writeVerdictProject(t, nil), false)

	project := writeVerdictProject(t, nil)
	t.Cleanup(func() { stopDelegatedDaemon(t, bin, home, project) })
	delegated := runSharedScan(t, bin, jar, home, project, true)

	if len(inProcess.Findings) == 0 {
		t.Fatal("the in-process scan reported nothing: the fixture is not exercising the checkers")
	}
	if strings.Join(delegated.Findings, "\n") != strings.Join(inProcess.Findings, "\n") {
		t.Fatalf("delegated findings differ from the in-process scan\n--- delegated\n%s\n--- in-process\n%s",
			strings.Join(delegated.Findings, "\n"), strings.Join(inProcess.Findings, "\n"))
	}
	// The cold delegated scan answered the checkers from the oracle's compile.
	if len(delegated.OutcomeLog) != 1 || delegated.OutcomeLog[0] != "shared" {
		t.Fatalf("firCheckBroker outcome = %v, want [shared]", delegated.OutcomeLog)
	}
}

// With an older krit-fir jar (KRIT_FIR_OLD_JAR, a jar from before the combined
// request) the delegated scan keeps the oracle's facts and reports what the
// in-process scan reports, compiling twice as before. Skipped without it.
func TestDelegatedFirScanWithAnOlderJarFallsBack(t *testing.T) {
	old := os.Getenv("KRIT_FIR_OLD_JAR")
	if old == "" {
		t.Skip("KRIT_FIR_OLD_JAR not set")
	}
	jar, err := filepath.Abs(old)
	if err != nil {
		t.Fatal(err)
	}
	bin := buildVerdictKrit(t, repoRoot(t))
	home := t.TempDir()
	inProcess := runSharedScan(t, bin, jar, home, writeVerdictProject(t, nil), false)
	project := writeVerdictProject(t, nil)
	t.Cleanup(func() { stopDelegatedDaemon(t, bin, home, project) })
	delegated := runSharedScan(t, bin, jar, home, project, true)
	if len(inProcess.Findings) == 0 || strings.Join(delegated.Findings, "\n") != strings.Join(inProcess.Findings, "\n") {
		t.Fatalf("delegated findings differ from in-process\n--- delegated\n%s\n--- in-process\n%s",
			strings.Join(delegated.Findings, "\n"), strings.Join(inProcess.Findings, "\n"))
	}
	if len(delegated.OutcomeLog) != 1 || delegated.OutcomeLog[0] == "shared" {
		t.Fatalf("firCheckBroker outcome = %v, want a decline", delegated.OutcomeLog)
	}
}
