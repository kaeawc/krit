package oracle

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/jvmaot"
	"github.com/kaeawc/krit/internal/jvmargs"
)

type discardWriteCloser struct{ io.Writer }

func (discardWriteCloser) Close() error { return nil }

// recordingDaemonBehindArgfile starts a stand-in process whose argv is the
// argfile form of a recording launch over jvmargs.Threshold, and returns a
// Daemon for it plus the temp profile, final profile, and build-lock paths.
func recordingDaemonBehindArgfile(t *testing.T, exitCode int) (*Daemon, string, string, string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	dir := t.TempDir()
	final := filepath.Join(dir, "krit-fir.aotconf")
	tmp := final + ".tmp." + strconv.Itoa(os.Getpid()) + ".1"
	lock := filepath.Join(dir, "krit-fir.aot.lock")
	if err := os.WriteFile(tmp, []byte("profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	launch := []string{"-XX:AOTMode=record", "-XX:AOTConfiguration=" + tmp, "-jar", "/krit-fir.jar", "--daemon"}
	launch = append(launch, "--classpath", strings.Repeat("/long/classpath/entry.jar:", jvmargs.Threshold/20))
	argv, cleanup, err := jvmargs.Prepare(launch)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if len(argv) != 1 || !strings.HasPrefix(argv[0], "@") {
		t.Fatalf("launch did not move into an argfile: %d args", len(argv))
	}
	cmd := exec.Command(sh, append([]string{"-c", "exit " + strconv.Itoa(exitCode), "java"}, argv...)...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if jvmaot.IsRecording(cmd.Args) {
		t.Fatal("test setup: cmd.Args should hide the recording flag behind the argfile")
	}
	d := &Daemon{
		cmd:        cmd,
		stdin:      discardWriteCloser{io.Discard},
		stdout:     bufio.NewScanner(strings.NewReader("")),
		nextID:     1,
		started:    true,
		launchArgs: launch,
	}
	return d, tmp, final, lock
}

// An argfile launch hides -XX:AOTConfiguration from cmd.Args. Release must
// still see the recording, shut the daemon down, and finalize the profile.
func TestReleaseFinalizesRecordingBehindArgfile(t *testing.T) {
	d, tmp, final, lock := recordingDaemonBehindArgfile(t, 0)
	if !jvmaot.IsRecording(d.jvmArgs()) {
		t.Fatal("recording launch behind an argfile is not seen as recording")
	}
	if err := d.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if d.started {
		t.Fatal("Release left a recording daemon running")
	}
	if data, err := os.ReadFile(final); err != nil || string(data) != "profile" {
		t.Fatalf("profile was not finalized: %q %v", data, err)
	}
	for _, p := range []string{tmp, lock} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s left behind: %v", p, err)
		}
	}
}

// A recording daemon that exits uncleanly discards its profile and still
// releases the build lock.
func TestShutdownDiscardsFailedRecordingBehindArgfile(t *testing.T) {
	d, tmp, final, lock := recordingDaemonBehindArgfile(t, 3)
	if err := d.Shutdown(); err == nil {
		t.Fatal("Shutdown: want the nonzero exit reported")
	}
	for _, p := range []string{tmp, final, lock} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s left behind: %v", p, err)
		}
	}
}

func TestDaemonJVMArgsFallsBackToCmdArgs(t *testing.T) {
	recording := []string{"java", "-XX:AOTMode=record", "-XX:AOTConfiguration=/cache/fir.aotconf.tmp.1.2", "-jar", "/fir.jar"}
	d := &Daemon{cmd: exec.Command(recording[0], recording[1:]...)}
	if !jvmaot.IsRecording(d.jvmArgs()) {
		t.Fatal("a daemon without launch args must fall back to cmd.Args")
	}
	if (&Daemon{}).jvmArgs() != nil {
		t.Fatal("a daemon without a command has no launch args")
	}
}
