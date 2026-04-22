package oracle

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// invokeTimeout returns the max wall-clock duration allowed for a krit-types
// one-shot run, from env KRIT_TYPES_TIMEOUT (default 15 minutes).
//
// The default was 5 minutes, which worked for small to medium repos but
// was right at the boundary for kotlin/kotlin cold runs (~5.1-5.4 min
// wall-clock quiescent). The timeout fired right before the jar wrote
// its output, producing a confusing "krit-types timed out" error on
// what was actually a perfectly healthy run. Bumped to 15 min so large
// mono-repo cold populates have comfortable headroom. The env var
// override remains for users with even larger corpora.
func invokeTimeout() time.Duration {
	if v := os.Getenv("KRIT_TYPES_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 15 * time.Minute
}

// invokeGraceExit returns how long to wait for the krit-types subprocess to
// exit on its own after it has written the output file, from env
// KRIT_TYPES_GRACE_EXIT (default 15 seconds). After this grace period the
// subprocess is force-killed — krit-types.jar sometimes fails to exit cleanly
// because Analysis API background threads keep the JVM alive, but the output
// file is already complete so it's safe to proceed.
func invokeGraceExit() time.Duration {
	if v := os.Getenv("KRIT_TYPES_GRACE_EXIT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return 15 * time.Second
}

// FindJar locates the krit-types shadow JAR by checking:
// 1. Next to the krit binary (tools/krit-types/build/libs/krit-types.jar)
// 2. In the project being scanned (.krit/krit-types.jar)
// 3. Relative to the krit binary's directory
func FindJar(scanPaths []string) string {
	candidates := []string{}

	// Check relative to the krit binary
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "krit-types.jar"),
			filepath.Join(exeDir, "tools", "krit-types", "build", "libs", "krit-types.jar"),
			filepath.Join(exeDir, "..", "tools", "krit-types", "build", "libs", "krit-types.jar"),
		)
	}

	// Check in the project directory
	if len(scanPaths) > 0 {
		projectDir := scanPaths[0]
		fi, err := os.Stat(projectDir)
		if err == nil && !fi.IsDir() {
			projectDir = filepath.Dir(projectDir)
		}
		candidates = append(candidates,
			filepath.Join(projectDir, ".krit", "krit-types.jar"),
			filepath.Join(projectDir, "tools", "krit-types", "build", "libs", "krit-types.jar"),
		)
	}

	// Check working directory
	cwd, _ := os.Getwd()
	candidates = append(candidates,
		filepath.Join(cwd, "tools", "krit-types", "build", "libs", "krit-types.jar"),
		filepath.Join(cwd, "krit-types.jar"),
	)

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// FindSourceDirs discovers Kotlin source directories under the given paths.
func FindSourceDirs(scanPaths []string) []string {
	var dirs []string
	seen := map[string]bool{}

	for _, root := range scanPaths {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				return nil
			}
			// Skip build/hidden directories
			base := filepath.Base(path)
			if base == "build" || base == ".gradle" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			// Check if this dir contains .kt files
			if info.Name() == "kotlin" || info.Name() == "java" {
				// Standard source layout: src/main/kotlin, src/commonMain/kotlin, etc.
				if !seen[path] {
					seen[path] = true
					dirs = append(dirs, path)
				}
				return filepath.SkipDir
			}
			return nil
		})
	}
	return dirs
}

// CachePath returns the path for the cached oracle JSON.
// Uses .krit/types.json in the first scan path directory.
func CachePath(scanPaths []string) string {
	if len(scanPaths) == 0 {
		return ""
	}
	projectDir := scanPaths[0]
	fi, err := os.Stat(projectDir)
	if err == nil && !fi.IsDir() {
		projectDir = filepath.Dir(projectDir)
	}
	return filepath.Join(projectDir, ".krit", "types.json")
}

// stderrTailSize is the maximum number of stderr bytes retained to attach to
// error messages when the oracle subprocess fails or times out. Anything
// beyond this cap is discarded from the tail but still streamed to the
// user's terminal when verbose mode is active.
const stderrTailSize = 8 * 1024

// stderrTail is an io.Writer that keeps the last stderrTailSize bytes it
// sees. Safe for concurrent writes from a single goroutine (the exec.Cmd
// stderr reader).
type stderrTail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *stderrTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > stderrTailSize {
		t.buf = t.buf[len(t.buf)-stderrTailSize:]
	}
	return len(p), nil
}

func (t *stderrTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// firstLines returns up to n newline-separated lines from s, joined back with
// single newlines. Used to keep oracle error messages compact — the full tail
// is still available for verbose debugging.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// Invoke runs krit-types.jar to produce a type oracle JSON file. It enforces
// a hard timeout (KRIT_TYPES_TIMEOUT, default 5m) and a soft grace period
// (KRIT_TYPES_GRACE_EXIT, default 15s) that starts once the output file
// appears on disk. The grace period exists because krit-types.jar has been
// observed to hang after writing its output when Analysis API background
// threads keep the JVM alive — once the output is complete we can force-kill
// the subprocess without losing work.
//
// On failure, the returned error includes the first few lines of the
// subprocess's captured stderr (up to 8 KB tail) so the caller can surface a
// diagnostic instead of a bare exit code. On success the output path is
// returned even when the grace period fired — the caller is responsible for
// validating the JSON via oracle.Load / oracle.LoadFromData.
func Invoke(jarPath string, sourceDirs []string, outputPath string, verbose bool) (string, error) {
	return InvokeWithFiles(jarPath, sourceDirs, outputPath, "", verbose)
}

// InvokeWithFiles is identical to Invoke but additionally passes a
// --files LISTFILE flag to krit-types if filesListPath is non-empty.
// LISTFILE is expected to contain one absolute .kt path per line and is
// produced by the rule-classification oracle filter
// (oracle.CollectOracleFiles + oracle.WriteFilterListFile). krit-types
// still builds the FIR session from the full --sources tree so that
// cross-file resolution works; the flag only narrows which files
// contribute expressions/declarations to the output JSON.
func InvokeWithFiles(jarPath string, sourceDirs []string, outputPath, filesListPath string, verbose bool) (string, error) {
	return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filesListPath, verbose, InvocationOptions{})
}

// InvokeWithFilesWithOptions is InvokeWithFiles plus optional perf
// instrumentation. The krit-types output schema stays unchanged; Kotlin-side
// timings are captured through a temporary --timings-out sidecar when a
// tracker is enabled.
func InvokeWithFilesWithOptions(jarPath string, sourceDirs []string, outputPath, filesListPath string, verbose bool, opts InvocationOptions) (string, error) {
	tracker := opts.tracker()
	// Check java is available
	var javaPath string
	if err := trackOracle(tracker, "javaLookup", func() error {
		var err error
		javaPath, err = exec.LookPath("java")
		if err != nil {
			return fmt.Errorf("java not found in PATH: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}

	// Ensure output directory exists
	if err := trackOracle(tracker, "outputDirCreate", func() error {
		return os.MkdirAll(filepath.Dir(outputPath), 0755)
	}); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	args := []string{
		// G1 is the default GC on JDK 21; explicit here so downstream tuning
		// is visible. String deduplication is a consistent 10-15% win on the
		// FQN-heavy FIR state that krit-types builds — Signal-Android sweep
		// measured 61.4s → 53.4s wall with +23% lower peak RSS (6.3→4.8 GB).
		// Skipping -Xmx so the kotlin/kotlin repo (32 GB peak) still fits;
		// -Xms1g avoids the early heap-grow pauses without over-committing.
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms1g",
		"-jar", jarPath,
		"--sources", strings.Join(sourceDirs, ","),
		"--output", outputPath,
	}
	if filesListPath != "" {
		args = append(args, "--files", filesListPath)
	}
	callFilterPath, cleanupCallFilter, err := writeCallFilterArg(opts, tracker)
	if err != nil {
		return "", fmt.Errorf("call filter: %w", err)
	}
	defer cleanupCallFilter()
	if callFilterPath != "" {
		args = append(args, "--call-filter", callFilterPath)
	}
	var cleanupTimings func()
	if tracker.IsEnabled() {
		timingsPath, cleanup, err := tempTimingsPath()
		if err != nil {
			return "", err
		}
		cleanupTimings = cleanup
		defer cleanupTimings()
		args = append(args, "--timings-out", timingsPath)
		defer addKotlinTimingsFromFile(tracker, timingsPath)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Running krit-types: %s %s\n", javaPath, strings.Join(args, " "))
	}

	timeout := invokeTimeout()
	graceExit := invokeGraceExit()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var res string
	processErr := trackOracle(tracker, "kritTypesProcess", func() error {
		var err error
		res, err = runOracleProcess(ctx, javaPath, args, outputPath, timeout, graceExit, verbose)
		return err
	})
	return res, processErr
}

// runOracleProcess is the exec+wait+grace-period+stderr-capture core shared by
// Invoke and the test harness. It's separated from Invoke to enable driving
// the full failure-mode matrix (clean exit, non-zero exit, hard timeout,
// grace-period force-kill) with cheap subprocess fixtures like `sh -c`
// instead of a real krit-types.jar.
func runOracleProcess(
	ctx context.Context,
	binaryPath string,
	args []string,
	outputPath string,
	timeout time.Duration,
	graceExit time.Duration,
	verbose bool,
) (string, error) {
	tail := &stderrTail{}
	var stderrWriter io.Writer = tail
	if verbose {
		stderrWriter = io.MultiWriter(tail, os.Stderr)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("krit-types start: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var outputSeenAt time.Time
	for {
		select {
		case err := <-done:
			if ctx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("krit-types timed out after %s\nstderr tail:\n%s",
					timeout, firstLines(tail.String(), 10))
			}
			if err != nil {
				return "", fmt.Errorf("krit-types failed: %w\nstderr tail:\n%s",
					err, firstLines(tail.String(), 10))
			}
			return outputPath, nil
		case <-ticker.C:
			if !outputSeenAt.IsZero() {
				if time.Since(outputSeenAt) >= graceExit {
					if verbose {
						fmt.Fprintf(os.Stderr, "verbose: krit-types wrote output but subprocess did not exit within %s grace period; force-killing\n", graceExit)
					}
					_ = cmd.Process.Kill()
					<-done // drain Wait() so exec doesn't leak the pipe
					return outputPath, nil
				}
				continue
			}
			if fi, err := os.Stat(outputPath); err == nil && fi.Size() > 0 {
				outputSeenAt = time.Now()
				if verbose {
					fmt.Fprintf(os.Stderr, "verbose: krit-types wrote %d bytes to %s; waiting up to %s for clean exit\n", fi.Size(), outputPath, graceExit)
				}
			}
		}
	}
}

// InvokeDaemon finds the JAR and source directories, then starts a long-lived
// krit-types daemon process. The caller is responsible for calling Close() on
// the returned Daemon.
func InvokeDaemon(scanPaths []string, verbose bool) (*Daemon, error) {
	jarPath := FindJar(scanPaths)
	if jarPath == "" {
		return nil, fmt.Errorf("krit-types.jar not found. Build it with: cd tools/krit-types && ./gradlew shadowJar")
	}

	sourceDirs := FindSourceDirs(scanPaths)
	if len(sourceDirs) == 0 {
		return nil, fmt.Errorf("no Kotlin source directories found")
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Found %d source directories for daemon\n", len(sourceDirs))
	}

	return StartDaemon(jarPath, sourceDirs, nil, verbose)
}

// InvokePersistentDaemon finds the JAR and source directories, then either
// connects to an existing persistent daemon or starts a new one. The daemon
// survives across krit invocations and is reused via PID file + TCP port.
// The caller is responsible for calling Close() on the returned Daemon.
func InvokePersistentDaemon(scanPaths []string, verbose bool) (*Daemon, error) {
	jarPath := FindJar(scanPaths)
	if jarPath == "" {
		return nil, fmt.Errorf("krit-types.jar not found. Build it with: cd tools/krit-types && ./gradlew shadowJar")
	}

	sourceDirs := FindSourceDirs(scanPaths)
	if len(sourceDirs) == 0 {
		return nil, fmt.Errorf("no Kotlin source directories found")
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Found %d source directories for persistent daemon\n", len(sourceDirs))
	}

	return ConnectOrStartDaemon(jarPath, sourceDirs, nil, verbose)
}
