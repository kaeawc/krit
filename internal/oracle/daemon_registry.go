package oracle

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/fslock"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/procstart"
)

// daemonCacheDir returns ~/.krit/cache/, creating it if needed.
func daemonCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".krit", "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

// daemonsDir returns the base for source-hash-named PID and port files.
// Tests can replace ~/.krit/cache/daemons with a private registry root.
func daemonsDir() (string, error) {
	dir := os.Getenv("KRIT_DAEMON_REGISTRY_DIR")
	mode := os.FileMode(0700)
	if dir == "" {
		base, err := daemonCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "daemons")
		mode = 0755
	}
	if err := os.MkdirAll(dir, mode); err != nil {
		return "", fmt.Errorf("create daemons dir: %w", err)
	}
	return dir, nil
}

// daemonPIDPathForSlot returns the path to the PID file for the daemon
// serving the given sourcesHash and slot. Each repo (identified by the
// hashSources() of its source directories) has its own PID file under
// ~/.krit/cache/daemons/{hash}.pid, so multiple daemons can coexist —
// one per repo the user is actively working on.
func daemonPIDPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonPIDFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonPIDFileName(sourcesHash, slot))
}

// daemonPortPathForSlot returns the path to the port file for the daemon
// serving the given sourcesHash and slot. Sibling of daemonPIDPathForSlot.
func daemonPortPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonPortFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonPortFileName(sourcesHash, slot))
}

func daemonPIDFileName(sourcesHash string, slot int) string {
	if slot <= 0 {
		return sourcesHash + ".pid"
	}
	return fmt.Sprintf("%s.%d.pid", sourcesHash, slot)
}

func daemonPortFileName(sourcesHash string, slot int) string {
	if slot <= 0 {
		return sourcesHash + ".port"
	}
	return fmt.Sprintf("%s.%d.port", sourcesHash, slot)
}

// daemonMetaPathForSlot returns the path to the identity sidecar written
// beside the PID and port files.
func daemonMetaPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonMetaFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonMetaFileName(sourcesHash, slot))
}

func daemonMetaFileName(sourcesHash string, slot int) string {
	if slot <= 0 {
		return sourcesHash + ".meta"
	}
	return fmt.Sprintf("%s.%d.meta", sourcesHash, slot)
}

// hashSources returns the 16-hex-char content-hash prefix of the
// sorted, newline-joined sourceDirs list. Deterministic across
// platforms — the sort makes the order of sourceDirs irrelevant to
// the hash, so "sourcesA\nsourcesB" and "sourcesB\nsourcesA" are the
// same repo. Relative dirs are hashed in absolute form: the registry is
// shared by every working directory, and "src/main/kotlin" names a
// different tree in each project.
func hashSources(sourceDirs []string) string {
	sorted := AbsolutePaths(sourceDirs)
	sort.Strings(sorted)
	return hashutil.HashHex([]byte(strings.Join(sorted, "\n")))[:16]
}

// daemonRegistryKey identifies a daemon slot by jar identity, source-dir
// set, and classpath, so a running krit-fir daemon isn't reused when a
// caller asks for krit-fir, and a daemon started with one classpath never
// answers for another (its facts would be computed against the wrong
// libraries).
func daemonRegistryKey(jarPath string, sourceDirs []string, classpath ...string) string {
	return daemonRegistryKeyKT(jarPath, sourceDirs, gradlemodel.KotlinSettings{}, classpath...)
}

// daemonRegistryKeyKT also keys the daemon on the Kotlin compiler settings it
// was launched with (`--kotlin-arg`), so a daemon started for one language
// level, opt-in set or flag set never answers for another. Zero settings give
// exactly the key daemonRegistryKey always produced.
func daemonRegistryKeyKT(jarPath string, sourceDirs []string, kt gradlemodel.KotlinSettings, classpath ...string) string {
	return daemonRegistryPrefix(jarPath, sourceDirs, kt, classpath...) + "@" + JarIdentity(jarPath)
}

func daemonRegistryPrefix(jarPath string, sourceDirs []string, kt gradlemodel.KotlinSettings, classpath ...string) string {
	return jarTag(jarPath) + "-" + JarPathTag(jarPath) + "-" + daemonLegacySuffix(sourceDirs, kotlinKeySuffix(kt), classpath...)
}

// kotlinKeySuffix is the registry key component for the settings, "" for none.
func kotlinKeySuffix(kt gradlemodel.KotlinSettings) string {
	if hash := kt.IdentityHash(); hash != "" {
		return "-kt" + hash
	}
	return ""
}

// DaemonOption configures a persistent oracle daemon beyond jar, sources and
// classpath.
type DaemonOption func(*daemonConfig)

type daemonConfig struct {
	kotlin          gradlemodel.KotlinSettings
	backend         Backend
	extraSourceDirs []string
}

// WithBackend records which oracle backend the daemon's jar serves. Callers
// know the backend they asked for; it is not inferred from the jar filename.
func WithBackend(b Backend) DaemonOption {
	return func(c *daemonConfig) { c.backend = b }
}

// WithKotlinSettings launches (or connects to) the daemon for the project's
// effective Kotlin compiler settings. Without it the embedded defaults apply.
func WithKotlinSettings(s gradlemodel.KotlinSettings) DaemonOption {
	return func(c *daemonConfig) { c.kotlin = s }
}

// WithExtraSourceDirs adds source roots beyond the conventional ones found
// under the scan paths (for example KMP source sets from the Gradle model)
// when InvokeDaemon* picks the daemon's source dirs.
func WithExtraSourceDirs(dirs []string) DaemonOption {
	return func(c *daemonConfig) { c.extraSourceDirs = dirs }
}

// BackendFromOptions reports the backend opts select (WithBackend), or ""
// when none is given. Lets callers that only forward options (and tests of
// them) check which backend a daemon will report.
func BackendFromOptions(opts ...DaemonOption) Backend {
	return resolveDaemonOptions(opts).backend
}

func resolveDaemonOptions(opts []DaemonOption) daemonConfig {
	var c daemonConfig
	for _, o := range opts {
		o(&c)
	}
	return c
}

func daemonLegacyKey(jarPath string, sourceDirs []string, classpath ...string) string {
	return jarTag(jarPath) + "-" + daemonLegacySuffix(sourceDirs, "", classpath...)
}

func daemonLegacySuffix(sourceDirs []string, kt string, classpath ...string) string {
	key := daemonClasspathPathPrefix(sourceDirs, kt, classpath...)
	if len(classpath) > 0 {
		key += "-" + gradlemodel.ClasspathFingerprint(AbsolutePaths(classpath))[:8]
	}
	return key
}

// The path-only prefix lets retirement find daemons whose classpath jars
// changed in place, while keeping unrelated classpaths separate.
func daemonClasspathPathPrefix(sourceDirs []string, kt string, classpath ...string) string {
	key := hashSources(sourceDirs) + kt
	if len(classpath) > 0 {
		// Order matters on a classpath, so hash it in the given order.
		key += "-" + hashutil.HashHex([]byte(strings.Join(AbsolutePaths(classpath), "\n")))[:8]
	}
	return key
}

func jarTag(jarPath string) string {
	if jarPath == "" {
		return "unknown"
	}
	return strings.TrimSuffix(strings.ToLower(filepath.Base(jarPath)), ".jar")
}

// pidFileInfo holds the contents of the PID file pair for one daemon.
type pidFileInfo struct {
	PID  int
	Port int
	// SourcesHash is the 16-hex-char hashSources() encoded in the
	// PID file's filename. Populated by readPIDFile from the hash
	// the caller looked up, so it's always set for successful reads.
	SourcesHash string
}

// readPIDFile reads the PID and port files for the daemon serving
// the given sourcesHash. Returns an error if either file is missing
// or unparseable. SourcesHash on the returned info is populated from
// the lookup key, not from disk.
func readPIDFile(sourcesHash string) (*pidFileInfo, error) {
	return readPIDFileSlot(sourcesHash, 0)
}

func readPIDFileSlot(sourcesHash string, slot int) (*pidFileInfo, error) {
	pidPath := daemonPIDPathForSlot(sourcesHash, slot)
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return nil, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return nil, fmt.Errorf("parse pid: %w", err)
	}

	portPath := daemonPortPathForSlot(sourcesHash, slot)
	portData, err := os.ReadFile(portPath)
	if err != nil {
		return nil, fmt.Errorf("read port file: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portData)))
	if err != nil {
		return nil, fmt.Errorf("parse port: %w", err)
	}

	return &pidFileInfo{PID: pid, Port: port, SourcesHash: sourcesHash}, nil
}

// writePIDFile records the PID and port for the daemon serving the
// given sourcesHash. Creates the daemons/ directory if needed.
func writePIDFile(pid, port int, sourcesHash string) error {
	return writePIDFileSlot(pid, port, sourcesHash, 0)
}

// writePIDFileSlot publishes a daemon under the per-key registry lock. It
// returns errDaemonRegistryBusy when the lock is not obtained and
// errDaemonRegistryOwned when another live daemon already holds the key; in
// both cases nothing was written and the caller keeps its daemon private.
func writePIDFileSlot(pid, port int, sourcesHash string, slot int) error {
	// Never write without the per-key lock: another publisher's port and PID
	// files would interleave with ours.
	release, ok := claimDaemonRegistry(sourcesHash, slot)
	if !ok {
		return errDaemonRegistryBusy
	}
	defer release()
	return writePIDFileSlotClaimed(pid, port, sourcesHash, slot)
}

// errDaemonBusy reports a registered daemon whose port accepted the
// connection but did not answer a ping in time. It is alive and, serving one
// client at a time, probably busy; it must not be stopped.
var errDaemonBusy = errors.New("daemon busy")

// errDaemonRegistryBusy reports that the per-key publication lock was not
// obtained (contention timeout or lock-file failure), so nothing was written.
var errDaemonRegistryBusy = errors.New("daemon registry publication lock not obtained")

// errDaemonRegistryOwned reports that another live daemon is already
// registered for the key, so nothing was written.
var errDaemonRegistryOwned = errors.New("daemon registry key already owned by another live daemon")

// writePIDFileSlotClaimed writes the record; the caller holds the claim. The
// identity metadata goes first, then the port, and the PID file is the
// discoverability marker written last; every file is replaced atomically
// (temp file + rename). A lock-free reader that sees a PID therefore sees a
// complete record, and never a torn file. Any previous PID file is removed
// first so a half-replaced record is never discoverable.
func writePIDFileSlotClaimed(pid, port int, sourcesHash string, slot int) error {
	// Re-read the registered PID now that the lock is held: another live
	// daemon already registered for this key is never displaced.
	if registered, found := registeredDaemonPID(sourcesHash, slot); found && registered != pid && daemonServes(sourcesHash, slot, registered) {
		return errDaemonRegistryOwned
	}
	pidPath := daemonPIDPathForSlot(sourcesHash, slot)
	_ = os.Remove(pidPath)
	if err := writeDaemonMeta(sourcesHash, slot, pid); err != nil {
		removeDaemonRegistryFiles(sourcesHash, slot)
		return err
	}
	if err := fsutil.WriteFileAtomic(daemonPortPathForSlot(sourcesHash, slot), []byte(strconv.Itoa(port)+"\n"), 0o644); err != nil {
		// Roll back so future invocations don't see an orphan record
		// pointing at a daemon the caller is about to kill.
		removeDaemonRegistryFiles(sourcesHash, slot)
		return fmt.Errorf("write port file: %w", err)
	}
	if err := fsutil.WriteFileAtomic(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		removeDaemonRegistryFiles(sourcesHash, slot)
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// daemonMeta is the identity sidecar persisted beside every registered
// daemon.
type daemonMeta struct {
	PID int `json:"pid"`
	// StartTime is the process start time of PID when it was registered. It
	// proves identity before the registry's PID is ever signalled: a PID
	// reused by an unrelated process has a different start time. Records
	// without it (written before it was recorded, or on a platform that
	// cannot read it) are unproven and their PID is never signalled.
	StartTime string `json:"startTime,omitempty"`
}

// daemonProcessStart reads a process's start time; tests replace it to
// simulate platforms where it is unreadable.
var daemonProcessStart = procstart.Of

// writeDaemonMeta records the start time of pid. Callers register a daemon
// they launched, so the live process is that daemon.
func writeDaemonMeta(sourcesHash string, slot int, pid int) error {
	data, err := json.Marshal(daemonMeta{PID: pid, StartTime: daemonProcessStart(pid)})
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(daemonMetaPathForSlot(sourcesHash, slot), data, 0o644); err != nil {
		return fmt.Errorf("write daemon metadata: %w", err)
	}
	return nil
}

// readDaemonMeta loads the record's metadata; ok is false when it is missing
// or unreadable.
func readDaemonMeta(sourcesHash string, slot int) (daemonMeta, bool) {
	var meta daemonMeta
	data, err := os.ReadFile(daemonMetaPathForSlot(sourcesHash, slot))
	if err != nil || json.Unmarshal(data, &meta) != nil {
		return daemonMeta{}, false
	}
	return meta, true
}

// daemonPIDProven reports whether pid is the process the registry recorded,
// by comparing the start time stored at publication with the live one.
func daemonPIDProven(sourcesHash string, slot int, pid int) bool {
	meta, ok := readDaemonMeta(sourcesHash, slot)
	if !ok || meta.PID != pid || meta.StartTime == "" {
		return false
	}
	return daemonProcessStart(pid) == meta.StartTime
}

func daemonLockPath(sourcesHash string, slot int) string {
	return daemonPIDPathForSlot(sourcesHash, slot) + ".lock"
}

// How long stopDaemonSlotIf waits for SIGTERM, then after SIGKILL, while it
// holds the per-key lock.
var (
	daemonStopTermWait = 5 * time.Second
	daemonStopKillWait = 3 * time.Second
)

// daemonClaimWait bounds how long anyone waits for the per-key lock. It must
// exceed the longest the lock is legitimately held, a stop's term+kill waits:
// otherwise every publisher queued behind a stop would give up and stay
// private. The margin covers the slower publish writes and scheduling delays.
var daemonClaimWait = daemonStopTermWait + daemonStopKillWait + 3*time.Second

// claimDaemonRegistry takes the per-key lock: an OS advisory lock, so exactly
// one process checks-and-publishes (or stops) at a time and the kernel
// releases it if the holder dies. It gives up after daemonClaimWait; the
// caller then stays private, or leaves the record alone.
func claimDaemonRegistry(sourcesHash string, slot int) (release func(), ok bool) {
	return fslock.Acquire(daemonLockPath(sourcesHash, slot), daemonClaimWait)
}

// registeredDaemonPID reads the PID the registry names for the key.
func registeredDaemonPID(sourcesHash string, slot int) (int, bool) {
	data, err := os.ReadFile(daemonPIDPathForSlot(sourcesHash, slot))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil && pid > 0
}

// daemonServes reports whether the registered PID is demonstrably the
// daemon: the process is alive AND the record's port accepts a connection. A
// bare live PID proves little, since a PID can be reused by an unrelated
// process after a crash or reboot; such a record is replaceable.
func daemonServes(sourcesHash string, slot int, pid int) bool {
	if !isProcessAlive(pid) {
		return false
	}
	data, err := os.ReadFile(daemonPortPathForSlot(sourcesHash, slot))
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port <= 0 {
		return false
	}
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// removeDaemonRegistryFiles deletes a record, the PID marker first so the
// record is never discoverable while half deleted. The caller holds the lock.
func removeDaemonRegistryFiles(sourcesHash string, slot int) {
	os.Remove(daemonPIDPathForSlot(sourcesHash, slot))
	os.Remove(daemonPortPathForSlot(sourcesHash, slot))
	os.Remove(daemonMetaPathForSlot(sourcesHash, slot))
}

// removePIDFile removes the PID and port files for the daemon
// serving the given sourcesHash. Silent on missing files.
func removePIDFile(sourcesHash string) {
	removePIDFileSlot(sourcesHash, 0)
}

// removePIDFileSlot deletes the slot's record under the per-key lock. If the
// lock cannot be taken the record is left alone: it is being published or
// replaced by someone else.
func removePIDFileSlot(sourcesHash string, slot int) {
	release, ok := claimDaemonRegistry(sourcesHash, slot)
	if !ok {
		return
	}
	defer release()
	removeDaemonRegistryFiles(sourcesHash, slot)
}

// removeDaemonRegistryIfOwned deletes the record under the lock, and only if
// the PID file still names pid, so a daemon never deletes the record of the
// successor that replaced it.
func removeDaemonRegistryIfOwned(sourcesHash string, slot int, pid int) {
	release, ok := claimDaemonRegistry(sourcesHash, slot)
	if !ok {
		return
	}
	defer release()
	if current, found := registeredDaemonPID(sourcesHash, slot); found && current == pid {
		removeDaemonRegistryFiles(sourcesHash, slot)
	}
}

// isProcessAlive checks whether a process with the given PID is running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 tests process existence without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// defaultDaemonPingTimeout bounds how long a client waits for an existing
// daemon to answer its reconnect ping.
const defaultDaemonPingTimeout = 5 * time.Second

// daemonPingTimeout is how long connectExistingDaemonSlotKT waits for the
// reconnect ping. KRIT_TYPES_REQUEST_TIMEOUT shortens it, never lengthens it.
func daemonPingTimeout() time.Duration {
	return min(defaultDaemonPingTimeout, daemonRequestTimeout())
}

// The dial retry loop smooths over transient socket races (TIME_WAIT,
// port-reuse) when reconnecting to a recently-restarted daemon. It does
// not mask a truly down daemon — the caller falls through to kill-stale
// + start-new on exhaustion.
const (
	dialDaemonAttempts  = 3
	dialDaemonBaseDelay = 100 * time.Millisecond
)

// dialDaemonSleep is the test seam for the retry loop's backoff.
var dialDaemonSleep = func(d time.Duration) { time.Sleep(d) }

func dialDaemonAddr(addr string, timeout time.Duration) (net.Conn, error) {
	var lastErr error
	for attempt := 1; attempt <= dialDaemonAttempts; attempt++ {
		c, err := (&net.Dialer{Timeout: timeout}).DialContext(context.Background(), "tcp", addr)
		if err == nil {
			return c, nil
		}
		lastErr = err
		if attempt == dialDaemonAttempts {
			break
		}
		dialDaemonSleep(dialDaemonBaseDelay << (attempt - 1))
	}
	return nil, lastErr
}

// connectDaemonTCP dials the daemon's TCP port and returns a buffered scanner over it.
// Removes the daemon's own registry record on connect failure.
func connectDaemonTCP(cmd *exec.Cmd, port int, srcHash string, slot int) (net.Conn, *bufio.Scanner, error) {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		cmd.Process.Kill()
		removeDaemonRegistryIfOwned(srcHash, slot, cmd.Process.Pid)
		return nil, nil, fmt.Errorf("connect to new daemon: %w", err)
	}
	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)
	return conn, reader, nil
}

// StartDaemon launches the krit-fir JVM process in daemon mode.
//
// If the JVM corrupts the JSON ready line with `[cds]`/`[aot]` log chatter
// (almost always because an AppCDS/Leyden archive in the per-jar cache was
// trained against a now-missing absolute classpath), the AppCDS+Leyden
// caches for this jar are purged and the spawn is retried once. The
// second attempt sees no cache files, so the JVM takes the training path
// and starts cleanly.
func StartDaemon(jarPath string, sourceDirs []string, classpath []string, verbose bool, opts ...DaemonOption) (*Daemon, error) {
	cfg := resolveDaemonOptions(opts)
	kt := cfg.kotlin
	d, err := retryOnHandshakeNoise(jarPath, verbose, func() (*Daemon, error) {
		return startDaemonOnce(jarPath, sourceDirs, classpath, verbose, kt)
	})
	if d != nil {
		d.backend = cfg.backend
	}
	return d, err
}

// retryOnHandshakeNoise runs spawn once, and if the daemon's handshake
// was overwritten by JVM unified-log chatter (a `handshakeNoiseError`),
// purges the per-jar AppCDS/Leyden caches and retries exactly once. Other
// errors (or success) are returned directly.
func retryOnHandshakeNoise(jarPath string, verbose bool, spawn func() (*Daemon, error)) (*Daemon, error) {
	d, err := spawn()
	var noise *handshakeNoiseError
	if !errors.As(err, &noise) {
		return d, err
	}
	if verbose {
		reporter().Verbosef("verbose: daemon handshake polluted by JVM log line; purging JVM caches and retrying: %s\n", noise.line)
	}
	purgeJVMCachesForJar(jarPath, verbose)
	return spawn()
}

// appendDaemonJarArgs appends the jar and its daemon inputs, all absolute:
// the daemon runs in DaemonWorkDir, not the caller's directory.
func appendDaemonJarArgs(args []string, jarPath string, sourceDirs, classpath []string, daemonArgs ...string) []string {
	return appendDaemonJarArgsKT(args, jarPath, sourceDirs, classpath, gradlemodel.KotlinSettings{}, daemonArgs...)
}

// appendDaemonJarArgsKT also launches the daemon under the Kotlin settings;
// each is its own `--kotlin-arg` flag, which both backends re-filter.
func appendDaemonJarArgsKT(args []string, jarPath string, sourceDirs, classpath []string, kt gradlemodel.KotlinSettings, daemonArgs ...string) []string {
	args = append(args, "-jar", AbsolutePath(jarPath))
	args = append(args, daemonArgs...)
	args = append(args, KotlinLaunchArgs(jarPath, kt)...)
	if len(sourceDirs) > 0 {
		args = append(args, "--sources", strings.Join(AbsolutePaths(sourceDirs), ","))
	}
	if len(classpath) > 0 {
		args = append(args, "--classpath", strings.Join(AbsolutePaths(classpath), string(os.PathListSeparator)))
	}
	return args
}

// EphemeralDaemonArgs scopes a newly spawned daemon to this client process.
// Normal CLI invocations leave the switch unset and retain shareable daemons.
func EphemeralDaemonArgs(args ...string) []string {
	if os.Getenv("KRIT_EPHEMERAL_DAEMONS") == "1" {
		return append(args, "--parent-pid", strconv.Itoa(os.Getpid()))
	}
	return args
}

func startDaemonOnce(jarPath string, sourceDirs []string, classpath []string, verbose bool, kt gradlemodel.KotlinSettings) (*Daemon, error) {
	// The daemon does not run in the caller's directory; the AOT/CDS
	// startup-cache args and -jar must name the same absolute jar.
	jarPath = AbsolutePath(jarPath)
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = appendDaemonJarArgsKT(args, jarPath, sourceDirs, classpath, kt, EphemeralDaemonArgs("--daemon")...)
	aotCache := startupAOTCache(args)

	if verbose {
		reporter().Verbosef("verbose: Starting krit-fir daemon: %s %s\n", javaPath, strings.Join(args, " "))
	}
	args, cleanupArgs, err := prepareJavaArgs(args)
	if err != nil {
		return nil, err
	}
	defer cleanupArgs()

	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	cmd.Dir = DaemonWorkDir()
	logFile := openDaemonLogFile(cmd, verbose)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	RecordTestDaemonPID(cmd.Process.Pid)

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

	d := &Daemon{
		cmd:          cmd,
		stdin:        stdinPipe,
		stdout:       scanner,
		logFile:      logFile,
		nextID:       1,
		sourceDirs:   sourceDirs,
		aotCachePath: aotCache,
		kotlin:       kt,
	}

	if _, err := waitPipeReady(cmd, scanner); err != nil {
		return nil, err
	}

	d.started = true
	if verbose {
		reporter().Verbosef("verbose: krit-fir daemon ready\n")
	}

	return d, nil
}

// connectExistingDaemonSlotKT attempts to connect to an already-running daemon
// for the given sourceDirs and Kotlin settings. Each repo has its own PID file
// under ~/.krit/cache/daemons/{hash}.{pid,port}, so multiple daemons (one
// per repo) can coexist under the same user cache hierarchy.
func connectExistingDaemonSlotKT(jarPath string, sourceDirs []string, kt gradlemodel.KotlinSettings, verbose bool, slot int, classpath ...string) (*Daemon, error) {
	hash := daemonRegistryKeyKT(jarPath, sourceDirs, kt, classpath...)
	info, err := readPIDFileSlot(hash, slot)
	if err != nil {
		return nil, fmt.Errorf("no existing daemon: %w", err)
	}

	if !isProcessAlive(info.PID) {
		return nil, fmt.Errorf("daemon PID %d is not alive", info.PID)
	}

	// Try connecting to the TCP port. Brief retry-with-backoff smooths
	// over transient socket races; on exhaustion the caller falls through
	// to kill-stale + start-new.
	conn, err := dialDaemonAddr(fmt.Sprintf("127.0.0.1:%d", info.Port), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon port %d: %w", info.Port, err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

	d := &Daemon{
		stdin:       conn,
		stdout:      reader,
		conn:        conn,
		port:        info.Port,
		nextID:      1,
		started:     true,
		shared:      true,
		slot:        slot,
		sourcesHash: info.SourcesHash,
		sourceDirs:  sourceDirs,
		kotlin:      kt,
	}

	// Verify the daemon is responsive with a ping
	if err := d.pingWithin(daemonPingTimeout()); err != nil {
		conn.Close()
		if errors.Is(err, errDaemonRequestTimeout) && daemonPIDProven(hash, slot, info.PID) {
			// The port accepted the connection but the answer is late: a
			// daemon serves one client at a time, so it is most likely busy
			// with another krit run. That is not a dead daemon. Only a PID
			// proven to be the registered daemon counts: a reused PID whose
			// old port another program now holds is a stale record to drop.
			return nil, fmt.Errorf("%w: %w", errDaemonBusy, err)
		}
		return nil, fmt.Errorf("daemon not responsive: %w", err)
	}

	if verbose {
		reporter().Verbosef("verbose: Reusing existing daemon slot %d (PID %d, port %d, sources %s)\n", slot, info.PID, info.Port, info.SourcesHash)
	}

	return d, nil
}

// cleanStaleDaemon detects and cleans up a stale daemon process for
// the given (jarPath, sourceDirs). Only touches the PID file entries
// belonging to this (jar, repo) pair — other registry entries are
// left alone.
func cleanStaleDaemon(jarPath string, sourceDirs []string, verbose bool, classpath ...string) {
	cleanStaleDaemonSlot(jarPath, sourceDirs, verbose, 0, classpath...)
}

func cleanStaleDaemonSlot(jarPath string, sourceDirs []string, verbose bool, slot int, classpath ...string) {
	cleanStaleDaemonSlotKT(jarPath, sourceDirs, gradlemodel.KotlinSettings{}, verbose, slot, classpath...)
}

func cleanStaleDaemonSlotKT(jarPath string, sourceDirs []string, kt gradlemodel.KotlinSettings, verbose bool, slot int, classpath ...string) {
	hash := daemonRegistryKeyKT(jarPath, sourceDirs, kt, classpath...)
	stopDaemonSlot(hash, slot, verbose)
}

// stopDaemonSlot stops whatever daemon the slot's record names. See
// stopDaemonSlotIf.
func stopDaemonSlot(hash string, slot int, verbose bool) {
	stopDaemonSlotIf(hash, slot, 0, verbose)
}

// stopDaemonSlotIf stops the daemon registered under hash and removes its
// record, but only under the per-key lock and, when expect is non-zero, only
// if the record still names expect. It reports whether it acted.
//
// Every destructive registry path goes through here (or
// removeDaemonRegistryIfOwned) so a publisher holding the lock is never raced:
// when the lock cannot be taken the record is treated as being published or
// replaced and nothing is killed or deleted. The caller then starts its own
// daemon, which publishes under the same lock or stays private.
func stopDaemonSlotIf(hash string, slot int, expect int, verbose bool) bool {
	release, ok := claimDaemonRegistry(hash, slot)
	if !ok {
		if verbose {
			reporter().Verbosef("verbose: daemon registry %s slot %d is locked by another process; not stopping its daemon\n", hash, slot)
		}
		return false
	}
	defer release()

	pid, found := registeredDaemonPID(hash, slot)
	if !found {
		// No PID marker means no discoverable daemon; clear any leftovers of
		// a publication that never completed.
		removeDaemonRegistryFiles(hash, slot)
		return true
	}
	if expect != 0 && pid != expect {
		if verbose {
			reporter().Verbosef("verbose: daemon registry %s slot %d now names PID %d, not %d; leaving it\n", hash, slot, pid, expect)
		}
		return false
	}
	if !isProcessAlive(pid) {
		if verbose {
			reporter().Verbosef("verbose: Cleaning up stale daemon slot %d PID file (PID %d no longer alive)\n", slot, pid)
		}
		removeDaemonRegistryFiles(hash, slot)
		return true
	}
	// Publication writes the port before the PID, so a record that names a
	// PID but no usable port is not one a publisher left behind: it is torn
	// or foreign. Drop it without signalling anything.
	if _, err := readPIDFileSlot(hash, slot); err != nil {
		if verbose {
			reporter().Verbosef("verbose: Dropping incomplete daemon registry %s slot %d without signalling PID %d: %v\n", hash, slot, pid, err)
		}
		removeDaemonRegistryFiles(hash, slot)
		return true
	}

	// A live PID alone proves nothing: after a crash or reboot it may belong
	// to an unrelated process of the same user. Signal it only when the start
	// time recorded at publication still matches; otherwise drop the record
	// and leave the process alone.
	if !daemonPIDProven(hash, slot, pid) {
		if verbose {
			reporter().Verbosef("verbose: Dropping daemon registry %s slot %d without signalling PID %d: cannot prove it is the daemon\n", hash, slot, pid)
		}
		removeDaemonRegistryFiles(hash, slot)
		return true
	}

	// Process is alive and is the registered daemon, but we couldn't use it
	// — it's stuck. Kill it.
	if verbose {
		reporter().Verbosef("verbose: Killing unresponsive daemon slot %d (PID %d)\n", slot, pid)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		removeDaemonRegistryFiles(hash, slot)
		return true
	}

	// SIGTERM, then SIGKILL after timeout. The PID file is only removed
	// once the process is confirmed dead — otherwise a follow-up
	// StartDaemonWithPort race could write a new PID file while a stale
	// JVM is still binding the same TCP port. waitUntilDead handles the
	// "SIGKILL needs more than a few hundred ms" case (loaded host,
	// kernel reaping lag) with a longer bounded loop.
	_ = proc.Signal(syscall.SIGTERM)
	if waitUntilDead(pid, daemonStopTermWait) {
		removeDaemonRegistryFiles(hash, slot)
		return true
	}

	if verbose {
		reporter().Verbosef("verbose: SIGTERM ignored by daemon slot %d (PID %d); sending SIGKILL\n", slot, pid)
	}
	_ = proc.Signal(syscall.SIGKILL)
	if waitUntilDead(pid, daemonStopKillWait) {
		removeDaemonRegistryFiles(hash, slot)
		return true
	}

	// SIGKILL didn't take effect within the bound — best-effort report
	// and drop the PID file anyway so future invocations don't get
	// wedged on it. The kernel will eventually reap; the worst case is
	// a brief window where the new daemon's port-0 bind dodges the
	// zombie's old port (port-0 always picks an unused one).
	reporter().Verbosef("verbose: daemon slot %d (PID %d) survived SIGKILL within timeout; PID file removed regardless\n", slot, pid)
	removeDaemonRegistryFiles(hash, slot)
	return true
}

// retireSupersededDaemons removes only entries for this path, sources and
// classpath. A concurrent request on an old daemon can be interrupted after
// its jar is replaced on disk; that daemon can no longer serve the current jar.
func retireSupersededDaemons(jarPath string, sourceDirs []string, classpath []string, verbose bool) {
	retireSupersededDaemonsKT(jarPath, sourceDirs, gradlemodel.KotlinSettings{}, classpath, verbose)
}

// retireSupersededDaemonsKT retires daemons of the same settings family whose
// classpath jars changed in place; daemons for other settings are left to
// serve their own requests and idle out.
func retireSupersededDaemonsKT(jarPath string, sourceDirs []string, kt gradlemodel.KotlinSettings, classpath []string, verbose bool) {
	dir, err := daemonsDir()
	if err != nil {
		return
	}
	current := daemonRegistryKeyKT(jarPath, sourceDirs, kt, classpath...)
	family := jarTag(jarPath) + "-" + JarPathTag(jarPath) + "-" + daemonClasspathPathPrefix(sourceDirs, kotlinKeySuffix(kt), classpath...)
	paths, err := filepath.Glob(filepath.Join(dir, family+"*.pid"))
	if err == nil {
		for _, path := range paths {
			stem := strings.TrimSuffix(filepath.Base(path), ".pid")
			candidatePrefix, _, found := strings.Cut(stem, "@")
			if !found || candidatePrefix != family && (len(classpath) == 0 || !strings.HasPrefix(candidatePrefix, family+"-")) {
				continue
			}
			key, slot, ok := parseDaemonPIDName(stem, candidatePrefix+"@")
			if ok && key != current {
				stopDaemonSlot(key, slot, verbose)
			}
		}
	}
	legacy := daemonLegacyKey(jarPath, sourceDirs, classpath...)
	if kotlinKeySuffix(kt) != "" {
		// Settings daemons never use the legacy key layout.
		return
	}
	paths, err = filepath.Glob(filepath.Join(dir, legacy+"*.pid"))
	if err == nil {
		for _, path := range paths {
			key, slot, ok := parseLegacyDaemonPIDName(strings.TrimSuffix(filepath.Base(path), ".pid"), legacy)
			if ok {
				stopDaemonSlot(key, slot, verbose)
			}
		}
	}
}

func parseDaemonPIDName(stem, prefix string) (string, int, bool) {
	if !strings.HasPrefix(stem, prefix) {
		return "", 0, false
	}
	remainder := strings.TrimPrefix(stem, prefix)
	identity, slotText, hasSlot := strings.Cut(remainder, ".")
	if identity != "missing" {
		if len(identity) != 8 {
			return "", 0, false
		}
		for _, c := range identity {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return "", 0, false
			}
		}
	}
	slot := 0
	if hasSlot {
		var err error
		slot, err = strconv.Atoi(slotText)
		if err != nil || slot < 1 || strconv.Itoa(slot) != slotText {
			return "", 0, false
		}
	}
	return prefix + identity, slot, true
}

func parseLegacyDaemonPIDName(stem, key string) (string, int, bool) {
	if stem == key {
		return key, 0, true
	}
	if !strings.HasPrefix(stem, key+".") {
		return "", 0, false
	}
	slotText := strings.TrimPrefix(stem, key+".")
	slot, err := strconv.Atoi(slotText)
	return key, slot, err == nil && slot > 0 && strconv.Itoa(slot) == slotText
}

// waitUntilDead polls isProcessAlive at 100ms intervals up to timeout.
// Returns true if the process is observed dead before timeout. Used by
// cleanStaleDaemonSlot to confirm SIGTERM/SIGKILL took effect before
// removing the PID file.
func waitUntilDead(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !isProcessAlive(pid)
}

// StartDaemonWithPort launches the krit-fir JVM process in daemon mode with
// a TCP listener. The daemon auto-assigns a port and reports it on stdout.
func StartDaemonWithPort(jarPath string, sourceDirs []string, classpath []string, verbose bool, opts ...DaemonOption) (*Daemon, error) {
	return StartDaemonWithPortSlot(jarPath, sourceDirs, classpath, verbose, 0, opts...)
}

func StartDaemonWithPortSlot(jarPath string, sourceDirs []string, classpath []string, verbose bool, slot int, opts ...DaemonOption) (*Daemon, error) {
	cfg := resolveDaemonOptions(opts)
	kt := cfg.kotlin
	d, err := retryOnHandshakeNoise(jarPath, verbose, func() (*Daemon, error) {
		return startDaemonWithPortSlotOnce(jarPath, sourceDirs, classpath, verbose, slot, kt)
	})
	if d != nil {
		d.backend = cfg.backend
	}
	return d, err
}

func startDaemonWithPortSlotOnce(jarPath string, sourceDirs []string, classpath []string, verbose bool, slot int, kt gradlemodel.KotlinSettings) (*Daemon, error) {
	// The daemon does not run in the caller's directory; the AOT/CDS
	// startup-cache args and -jar must name the same absolute jar.
	jarPath = AbsolutePath(jarPath)
	// Capture the jar identity before the JVM opens the jar. If the jar is
	// replaced during startup, the daemon is registered under the identity
	// observed first, so the next caller sees a mismatch and restarts rather
	// than trusting a daemon that may be running the old artifact.
	srcHash := daemonRegistryKeyKT(jarPath, sourceDirs, kt, classpath...)

	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = appendDaemonJarArgsKT(args, jarPath, sourceDirs, classpath, kt, EphemeralDaemonArgs("--daemon", "--port", "0")...)
	aotCache := startupAOTCache(args)

	if verbose {
		reporter().Verbosef("verbose: Starting persistent krit-fir daemon slot %d: %s %s\n", slot, javaPath, strings.Join(args, " "))
	}
	args, cleanupArgs, err := prepareJavaArgs(args)
	if err != nil {
		return nil, err
	}
	defer cleanupArgs()

	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	cmd.Dir = DaemonWorkDir()
	logFile := openDaemonLogFile(cmd, verbose)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	RecordTestDaemonPID(cmd.Process.Pid)

	ready, err := waitPortReady(cmd, stdoutPipe)
	if err != nil {
		return nil, err
	}

	if verbose {
		reporter().Verbosef("verbose: Daemon started on port %d (PID %d)\n", ready.Port, cmd.Process.Pid)
	}

	unpublished := false
	if err := writePIDFileSlot(cmd.Process.Pid, ready.Port, srcHash, slot); err != nil {
		if errors.Is(err, errDaemonRegistryBusy) || errors.Is(err, errDaemonRegistryOwned) {
			// The key is being published by, or already belongs to, another
			// process. Leave its record alone and keep this daemon private.
			if verbose {
				reporter().Verbosef("verbose: daemon registry for %s slot %d is locked or owned by another process (%v); keeping the daemon private\n", srcHash, slot, err)
			}
			unpublished = true
		} else {
			cmd.Process.Kill()
			// writePIDFileSlot already rolls back its partial write, but
			// remove any record naming this PID too, so nothing is left
			// pointing at a daemon we just killed.
			removeDaemonRegistryIfOwned(srcHash, slot, cmd.Process.Pid)
			return nil, fmt.Errorf("write PID file: %w", err)
		}
	}

	conn, reader, err := connectDaemonTCP(cmd, ready.Port, srcHash, slot)
	if err != nil {
		return nil, err
	}

	d := &Daemon{
		cmd:          cmd,
		stdin:        conn,
		stdout:       reader,
		conn:         conn,
		logFile:      logFile,
		port:         ready.Port,
		nextID:       1,
		started:      true,
		shared:       false,
		unpublished:  unpublished,
		slot:         slot,
		sourcesHash:  srcHash,
		sourceDirs:   sourceDirs,
		aotCachePath: aotCache,
		kotlin:       kt,
	}

	return d, nil
}

// startDaemonWithPortSlot launches a registered daemon; tests replace it to
// avoid a real JVM.
var startDaemonWithPortSlot = StartDaemonWithPortSlot

// ConnectOrStartDaemon tries to reuse an existing persistent daemon
// for the given sourceDirs. Each repo has its own PID file under
// ~/.krit/cache/daemons/{hash}.{pid,port} so multiple daemons (one per
// repo) can coexist; krit invocations targeting different repos don't
// fight over a shared PID file. If no daemon is running (or the
// existing one is unresponsive), cleanStaleDaemon wipes just this
// repo's entry and StartDaemonWithPort starts a fresh one.
func ConnectOrStartDaemon(jarPath string, sourceDirs []string, classpath []string, verbose bool, opts ...DaemonOption) (*Daemon, error) {
	cfg := resolveDaemonOptions(opts)
	kt := cfg.kotlin
	// Try connecting to an existing daemon for this (jar, source tree)
	// pair. The jar is part of the registry key so a running
	// krit-fir daemon isn't reused when the caller asks for
	// krit-fir (or vice versa).
	hash := daemonRegistryKeyKT(jarPath, sourceDirs, kt, classpath...)
	// rejected is the PID of the record this call decides against. The stop
	// below re-validates it under the registry lock, so a record another
	// process published in the meantime is never killed.
	rejected, _ := registeredDaemonPID(hash, 0)
	d, err := connectExistingDaemonSlotKT(jarPath, sourceDirs, kt, verbose, 0, classpath...)
	if err == nil {
		d.backend = cfg.backend
		return d, nil
	}
	if errors.Is(err, errDaemonBusy) {
		// A busy daemon is serving another run. Leave it registered and
		// running, and use a private daemon for this run instead: stopping it
		// would kill the other run's request and lose its JVM warm-up.
		if verbose {
			reporter().Verbosef("verbose: krit-fir daemon for %s is busy; starting a private daemon for this run\n", hash)
		}
		rejected = 0
	}
	retireSupersededDaemonsKT(jarPath, sourceDirs, kt, classpath, verbose)

	// Clean up this (jar, repo)'s stale daemon entry (if any). Other
	// registry entries are left alone.
	if rejected != 0 {
		stopDaemonSlotIf(hash, 0, rejected, verbose)
	}

	// Start a new persistent daemon. If a busy one still holds the key, the
	// new daemon cannot publish and stays private to this process.
	d, err = startDaemonWithPortSlot(jarPath, sourceDirs, classpath, verbose, 0, WithKotlinSettings(kt), WithBackend(cfg.backend))
	if err != nil {
		return nil, fmt.Errorf("start persistent daemon: %w", err)
	}

	return d, nil
}
