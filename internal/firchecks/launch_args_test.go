package firchecks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/jvmaot"
)

// linuxMaxArgStrlen is Linux's MAX_ARG_STRLEN: the most bytes one execve
// argument may hold, whatever ARG_MAX is.
const linuxMaxArgStrlen = 128 * 1024

func TestFirDaemonArgvManyPluginJarsUseArgfile(t *testing.T) {
	dir := t.TempDir()
	jars := make([]string, 100_001)
	for i := range jars {
		jars[i] = filepath.Join(dir, fmt.Sprintf("plugin-%06d.jar", i))
	}
	target := CompileTarget{Kotlin: gradlemodel.KotlinSettings{Plugins: gradlemodel.CompilerPlugins{Classpath: jars}}}
	args := firDaemonLaunchArgs(filepath.Join(dir, "krit-fir.jar"), "missing-java", 27, false, firAOTOff, target)
	if len(args) < 2*len(jars) {
		t.Fatalf("launch args hold %d entries, want every plugin jar", len(args))
	}
	assertFirArgvSingleArgfile(t, args)
}

func TestFirDaemonArgvLongArgumentUseArgfile(t *testing.T) {
	dir := t.TempDir()
	option := "plugin:dev.example:packages=" + strings.Repeat("com.example.module.", linuxMaxArgStrlen/19+1)
	target := CompileTarget{Kotlin: gradlemodel.KotlinSettings{Plugins: gradlemodel.CompilerPlugins{
		Classpath: []string{filepath.Join(dir, "plugin.jar")},
		Options:   []string{option},
	}}}
	args := firDaemonLaunchArgs(filepath.Join(dir, "krit-fir.jar"), "missing-java", 27, false, firAOTOff, target)
	if !slices.Contains(args, option) || len(option) <= linuxMaxArgStrlen {
		t.Fatalf("launch args do not carry the %d-byte plugin option", len(option))
	}
	assertFirArgvSingleArgfile(t, args)
}

func TestFirDaemonArgvShortArgsUnchanged(t *testing.T) {
	dir := t.TempDir()
	args := firDaemonLaunchArgs(filepath.Join(dir, "krit-fir.jar"), "missing-java", 27, false, firAOTOff, CompileTarget{JvmTarget: "17"})
	got, cleanup, err := prepareFirDaemonArgv(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !slices.Equal(got, args) {
		t.Fatalf("short argv changed:\n got %q\nwant %q", got, args)
	}
}

// An argfile launch hides -XX:AOTConfiguration from cmd.Args, so the
// recording checks must read the launch arguments instead.
func TestFirDaemonJVMArgsSeeRecordingBehindArgfile(t *testing.T) {
	recording := []string{"-XX:AOTMode=record", "-XX:AOTConfiguration=/cache/fir.aotconf.tmp.123", "-jar", "/fir.jar"}
	d := &FirDaemon{cmd: exec.Command("java", "@/tmp/krit-java-1.args"), launchArgs: recording}
	if !jvmaot.IsRecording(d.jvmArgs()) {
		t.Fatal("recording launch behind an argfile is not seen as recording")
	}
	plain := &FirDaemon{cmd: exec.Command("java", recording...)}
	if !jvmaot.IsRecording(plain.jvmArgs()) {
		t.Fatal("a daemon without launch args must fall back to cmd.Args")
	}
}

func assertFirArgvSingleArgfile(t *testing.T, args []string) {
	t.Helper()
	got, cleanup, err := prepareFirDaemonArgv(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") {
		t.Fatalf("argv is not a single @argfile: %d args", len(got))
	}
	body, err := os.ReadFile(strings.TrimPrefix(got[0], "@"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n"); !slices.Equal(lines, args) {
		t.Fatalf("argfile holds %d args, want %d (or contents differ)", len(lines), len(args))
	}
}
