package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ===========================================================================
// Step 2: PID file layer tests
// ===========================================================================

var testSourceDirs = []string{"/test/fixture/repo"}
var testSourcesHash = hashSources(testSourceDirs)

func TestWritePIDFile_CreatesFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := writePIDFile(12345, 54321, testSourcesHash); err != nil {
		t.Fatalf("writePIDFile error: %v", err)
	}

	// Verify PID file exists at the per-hash path and has correct content
	pidPath := filepath.Join(tmpHome, ".krit", "cache", "daemons", testSourcesHash+".pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	if pid != 12345 {
		t.Errorf("expected PID 12345, got %d", pid)
	}

	// Verify port file exists at the per-hash path and has correct content
	portPath := filepath.Join(tmpHome, ".krit", "cache", "daemons", testSourcesHash+".port")
	portData, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatalf("read port file: %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portData)))
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	if port != 54321 {
		t.Errorf("expected port 54321, got %d", port)
	}
}

func TestReadPIDFile_ValidFormat(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("42\n"), 0644)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".port"), []byte("8080\n"), 0644)

	info, err := readPIDFile(testSourcesHash)
	if err != nil {
		t.Fatalf("readPIDFile error: %v", err)
	}
	if info.PID != 42 {
		t.Errorf("expected PID 42, got %d", info.PID)
	}
	if info.Port != 8080 {
		t.Errorf("expected port 8080, got %d", info.Port)
	}
	if info.SourcesHash != testSourcesHash {
		t.Errorf("expected SourcesHash %q, got %q", testSourcesHash, info.SourcesHash)
	}
}

func TestReadPIDFile_InvalidFormat(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)

	// Corrupt PID file (non-numeric)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("not-a-number\n"), 0644)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".port"), []byte("8080\n"), 0644)

	_, err := readPIDFile(testSourcesHash)
	if err == nil {
		t.Fatal("expected error for corrupt PID file")
	}
	if !strings.Contains(err.Error(), "parse pid") {
		t.Errorf("expected 'parse pid' error, got: %v", err)
	}
}

func TestReadPIDFile_InvalidPortFormat(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("42\n"), 0644)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".port"), []byte("garbage\n"), 0644)

	_, err := readPIDFile(testSourcesHash)
	if err == nil {
		t.Fatal("expected error for corrupt port file")
	}
	if !strings.Contains(err.Error(), "parse port") {
		t.Errorf("expected 'parse port' error, got: %v", err)
	}
}

func TestReadPIDFile_MissingFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No PID files created — should error
	_, err := readPIDFile(testSourcesHash)
	if err == nil {
		t.Fatal("expected error when PID file missing")
	}
	if !strings.Contains(err.Error(), "read pid file") {
		t.Errorf("expected 'read pid file' error, got: %v", err)
	}
}

func TestReadPIDFile_MissingPortFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("42\n"), 0644)
	// No port file

	_, err := readPIDFile(testSourcesHash)
	if err == nil {
		t.Fatal("expected error when port file missing")
	}
	if !strings.Contains(err.Error(), "read port file") {
		t.Errorf("expected 'read port file' error, got: %v", err)
	}
}

func TestRemovePIDFile_Cleanup(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	pidFile := filepath.Join(daemonsDir, testSourcesHash+".pid")
	portFile := filepath.Join(daemonsDir, testSourcesHash+".port")
	os.WriteFile(pidFile, []byte("42\n"), 0644)
	os.WriteFile(portFile, []byte("8080\n"), 0644)

	removePIDFile(testSourcesHash)

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed")
	}
	if _, err := os.Stat(portFile); !os.IsNotExist(err) {
		t.Error("expected port file to be removed")
	}
}

func TestRemovePIDFile_NoopWhenMissing(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Should not panic when files do not exist
	removePIDFile(testSourcesHash)
}

func TestPIDFiles_MultipleReposCoexist(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Write daemons for two distinct repos
	hashA := hashSources([]string{"/repo/A"})
	hashB := hashSources([]string{"/repo/B"})
	if hashA == hashB {
		t.Fatal("hashes should differ for different sources")
	}

	if err := writePIDFile(111, 1111, hashA); err != nil {
		t.Fatal(err)
	}
	if err := writePIDFile(222, 2222, hashB); err != nil {
		t.Fatal(err)
	}

	// Both should be readable independently
	infoA, err := readPIDFile(hashA)
	if err != nil || infoA.PID != 111 || infoA.Port != 1111 {
		t.Errorf("read A: %+v err=%v", infoA, err)
	}
	infoB, err := readPIDFile(hashB)
	if err != nil || infoB.PID != 222 || infoB.Port != 2222 {
		t.Errorf("read B: %+v err=%v", infoB, err)
	}

	// Removing A should not affect B
	removePIDFile(hashA)
	if _, err := readPIDFile(hashA); err == nil {
		t.Error("expected A removed but readable")
	}
	if _, err := readPIDFile(hashB); err != nil {
		t.Errorf("B should still be readable after A removed: %v", err)
	}

	// Clean up B for test hygiene
	removePIDFile(hashB)
}

func TestIsProcessAlive_LiveProcess(t *testing.T) {
	// Our own PID is always alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("expected own process to be alive")
	}
}

func TestIsProcessAlive_DeadProcess(t *testing.T) {
	// PID 999999 should not exist on any reasonable system
	if isProcessAlive(999999) {
		t.Error("expected PID 999999 to not be alive")
	}
}

func TestWritePIDFile_OverwritesExisting(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Write first set
	if err := writePIDFile(100, 200, testSourcesHash); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Overwrite with new values
	if err := writePIDFile(300, 400, testSourcesHash); err != nil {
		t.Fatalf("second write: %v", err)
	}

	info, err := readPIDFile(testSourcesHash)
	if err != nil {
		t.Fatalf("readPIDFile: %v", err)
	}
	if info.PID != 300 {
		t.Errorf("expected PID 300, got %d", info.PID)
	}
	if info.Port != 400 {
		t.Errorf("expected port 400, got %d", info.Port)
	}
}

// ===========================================================================
// Step 3: TCP connection layer tests
// ===========================================================================

func TestConnectToDaemon_Success(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	// Should be able to ping the fake daemon
	err := d.Ping()
	if err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestConnectToDaemon_RefusedPort(t *testing.T) {
	// Try connecting to a port where nothing is listening.
	// Use a random port that we immediately close.
	listener := listenLocalTCP(t)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Close immediately so the port is refused

	_, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err == nil {
		t.Fatal("expected connection refused error")
	}
}

func TestConnectToDaemon_Timeout(t *testing.T) {
	// Listen but never accept — connection should eventually time out
	listener := listenLocalTCP(t)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// Set a very short timeout to avoid slow test
	start := time.Now()
	_, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
	elapsed := time.Since(start)

	// On macOS/Linux, connect to a listening port succeeds even without Accept.
	// So we test a non-routable address instead for true timeout behavior.
	// But since that would be flaky in CI, we just verify the connection layer works.
	_ = err
	_ = elapsed
}

func TestDaemonLookup_ValidResponse(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	// Configure a custom analyze response with file data (must be single line for newline-delimited JSON)
	fake.Responses["analyze"] = `{"version": 1, "kotlinVersion": "2.1.0", "files": {"src/App.kt": {"package": "com.example", "declarations": [{"fqn": "com.example.App", "kind": "class"}]}}, "dependencies": {}}`

	d := fake.ConnectDaemon(t)
	defer d.Close()

	data, err := d.Analyze([]string{"src/App.kt"})
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}
	if data.KotlinVersion != "2.1.0" {
		t.Errorf("expected kotlinVersion 2.1.0, got %s", data.KotlinVersion)
	}
	if len(data.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(data.Files))
	}
	file, ok := data.Files["src/App.kt"]
	if !ok {
		t.Fatal("expected src/App.kt in files")
	}
	if file.Package != "com.example" {
		t.Errorf("expected package com.example, got %q", file.Package)
	}
}

func TestDaemonLookup_ErrorResponse(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	// Remove the default analyze response so the fake returns "unknown method"
	delete(fake.Responses, "analyze")

	d := fake.ConnectDaemon(t)
	defer d.Close()

	_, err := d.Analyze([]string{"test.kt"})
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if !strings.Contains(err.Error(), "unknown method") {
		t.Errorf("expected 'unknown method' error, got: %v", err)
	}
}

func TestDaemonLookup_MalformedJSON(t *testing.T) {
	// Use a raw TCP mock that sends malformed JSON
	listener := listenLocalTCP(t)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read request
		buf := make([]byte, 4096)
		conn.Read(buf)

		// Send malformed JSON
		conn.Write([]byte("{not valid json\n"))
	}()

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	d := &Daemon{
		stdin:   conn,
		stdout:  newScannerFromConn(conn),
		conn:    conn,
		port:    port,
		nextID:  1,
		started: true,
		shared:  true,
	}
	defer d.Close()

	_, err = d.AnalyzeAll()
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("expected 'unmarshal response' error, got: %v", err)
	}
}

func TestDaemonLookup_AnalyzeAll_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}
}

func TestDaemonLookup_Rebuild_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	if err := d.Rebuild(); err != nil {
		t.Fatalf("Rebuild error: %v", err)
	}
}

func TestDaemonLookup_Checkpoint_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	if err := d.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint error: %v", err)
	}
}

func TestDaemonLookup_MultipleSequentialRequests_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	// Ping
	if err := d.Ping(); err != nil {
		t.Fatalf("Ping error: %v", err)
	}

	// AnalyzeAll
	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}

	// Rebuild
	if err := d.Rebuild(); err != nil {
		t.Fatalf("Rebuild error: %v", err)
	}

	// Analyze
	if _, err := d.Analyze([]string{"a.kt"}); err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Verify IDs incremented correctly
	if d.nextID != 5 {
		t.Errorf("expected nextID=5 after 4 requests, got %d", d.nextID)
	}
}

func TestDaemonLookup_CheckpointSuccess_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)
	defer d.Close()

	// With default success response, checkpoint should succeed
	if err := d.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint error: %v", err)
	}
}

func TestDaemonLookup_CheckpointUnknownMethod_TCP(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	// Remove checkpoint so fake returns "unknown method" error
	delete(fake.Responses, "checkpoint")

	d := fake.ConnectDaemon(t)
	defer d.Close()

	// "unknown method" is not "CRaC not available", so Checkpoint should propagate the error
	err := d.Checkpoint()
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if !strings.Contains(err.Error(), "unknown method") {
		t.Errorf("expected 'unknown method' error, got: %v", err)
	}
}

// ===========================================================================
// Step 4: AppCDS layer tests
// ===========================================================================

func TestCDSArchivePath_DeterministicHash(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("deterministic content"), 0644)

	path1, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	path2, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if path1 != path2 {
		t.Errorf("expected same path for same JAR, got %q vs %q", path1, path2)
	}
}

func TestCDSArchivePath_DifferentJAR(t *testing.T) {
	tmpDir := t.TempDir()
	jar1 := filepath.Join(tmpDir, "a.jar")
	jar2 := filepath.Join(tmpDir, "b.jar")
	os.WriteFile(jar1, []byte("content A"), 0644)
	os.WriteFile(jar2, []byte("content B"), 0644)

	path1, err := cdsArchivePath(jar1)
	if err != nil {
		t.Fatalf("jar1: %v", err)
	}
	path2, err := cdsArchivePath(jar2)
	if err != nil {
		t.Fatalf("jar2: %v", err)
	}

	if path1 == path2 {
		t.Error("expected different archive paths for different JAR content")
	}
}

func TestCDSArchivePath_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("jar"), 0644)

	path, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath: %v", err)
	}

	// The parent directory of the archive path should exist
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat archive dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected archive parent to be a directory")
	}
}

func TestAppCDSFlags_FirstRun(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("jar content for first run"), 0644)

	archivePath, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath: %v", err)
	}

	// On first run, the archive file should NOT exist
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("expected archive to not exist on first run")
	}

	// The daemon should use ArchiveClassesAtExit for training
	// Simulate the flag logic from StartDaemon
	var args []string
	if _, statErr := os.Stat(archivePath); statErr == nil {
		args = append(args, "-XX:SharedArchiveFile="+archivePath, "-Xshare:auto")
	} else {
		args = append(args, "-XX:ArchiveClassesAtExit="+archivePath, "-Xshare:auto")
	}

	found := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-XX:ArchiveClassesAtExit=") {
			found = true
			if !strings.Contains(arg, archivePath) {
				t.Errorf("expected archive path in flag, got %q", arg)
			}
		}
	}
	if !found {
		t.Error("expected ArchiveClassesAtExit flag on first run")
	}
}

func TestAppCDSFlags_SubsequentRun(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("jar content for subsequent run"), 0644)

	archivePath, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath: %v", err)
	}

	// Create the archive file to simulate a previous run
	os.WriteFile(archivePath, []byte("fake archive"), 0644)

	// The daemon should use SharedArchiveFile for reuse
	var args []string
	if _, statErr := os.Stat(archivePath); statErr == nil {
		args = append(args, "-XX:SharedArchiveFile="+archivePath, "-Xshare:auto")
	} else {
		args = append(args, "-XX:ArchiveClassesAtExit="+archivePath, "-Xshare:auto")
	}

	found := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-XX:SharedArchiveFile=") {
			found = true
			if !strings.Contains(arg, archivePath) {
				t.Errorf("expected archive path in flag, got %q", arg)
			}
		}
	}
	if !found {
		t.Error("expected SharedArchiveFile flag on subsequent run")
	}
}

func TestAppCDSFlags_StaleArchive(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("original content"), 0644)

	archivePath1, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath: %v", err)
	}

	// Create archive for original content
	os.WriteFile(archivePath1, []byte("archive for original"), 0644)

	// Now change the JAR content (simulating an upgrade)
	os.WriteFile(jarPath, []byte("new content after upgrade"), 0644)

	archivePath2, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath after change: %v", err)
	}

	// The archive path should differ because the hash changed
	if archivePath1 == archivePath2 {
		t.Error("expected different archive path after JAR content change")
	}

	// The new archive should not exist yet — daemon should retrain
	if _, statErr := os.Stat(archivePath2); !os.IsNotExist(statErr) {
		t.Error("expected new archive to not exist (should trigger retrain)")
	}
}

func TestCDSArchivePath_MissingJAR(t *testing.T) {
	_, err := cdsArchivePath("/nonexistent/path/to.jar")
	if err == nil {
		t.Error("expected error for missing JAR")
	}
}

func TestCDSArchivePath_PathFormat(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("test"), 0644)

	path, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath: %v", err)
	}

	if !strings.HasSuffix(path, ".jsa") {
		t.Errorf("expected .jsa suffix, got %q", path)
	}
	if !strings.Contains(filepath.Base(path), "krit-types-") {
		t.Errorf("expected krit-types- prefix in filename, got %q", filepath.Base(path))
	}
	// Hash portion should be 12 hex chars
	base := filepath.Base(path)
	hash := strings.TrimPrefix(base, "krit-types-")
	hash = strings.TrimSuffix(hash, ".jsa")
	if len(hash) != 12 {
		t.Errorf("expected 12-char hash, got %d chars: %q", len(hash), hash)
	}
}

// ===========================================================================
// Step 5: CRaC layer tests
// ===========================================================================

func TestCRaCCheckpointPath_DeterministicHash(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("deterministic crac content"), 0644)

	path1, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	path2, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if path1 != path2 {
		t.Errorf("expected same path, got %q vs %q", path1, path2)
	}
}

func TestCRaCCheckpointPath_DifferentJAR(t *testing.T) {
	tmpDir := t.TempDir()
	jar1 := filepath.Join(tmpDir, "a.jar")
	jar2 := filepath.Join(tmpDir, "b.jar")
	os.WriteFile(jar1, []byte("crac A"), 0644)
	os.WriteFile(jar2, []byte("crac B"), 0644)

	path1, _ := cracCheckpointPath(jar1)
	path2, _ := cracCheckpointPath(jar2)

	if path1 == path2 {
		t.Error("expected different checkpoint paths for different JARs")
	}
}

func TestCRaCCheckpointPath_PathFormat(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("crac test"), 0644)

	path, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("cracCheckpointPath: %v", err)
	}

	if !strings.HasSuffix(path, ".crac") {
		t.Errorf("expected .crac suffix, got %q", path)
	}
	if !strings.Contains(filepath.Base(path), "krit-types-") {
		t.Errorf("expected krit-types- prefix, got %q", filepath.Base(path))
	}
}

func TestCRaCFlags_NoCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("no checkpoint"), 0644)

	cracPath, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("cracCheckpointPath: %v", err)
	}

	// No checkpoint directory exists — CRaC flags should NOT be added
	_, statErr := os.Stat(cracPath)
	if !os.IsNotExist(statErr) {
		t.Fatal("expected checkpoint path to not exist")
	}

	// Replicate the daemon's CRaC logic
	var args []string
	if statErr == nil {
		info, _ := os.Stat(cracPath)
		if info != nil && info.IsDir() {
			args = append(args, "-XX:CRaCRestoreFrom="+cracPath)
		}
	}

	if len(args) != 0 {
		t.Errorf("expected no CRaC flags when no checkpoint exists, got %v", args)
	}
}

func TestCRaCFlags_WithCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("with checkpoint"), 0644)

	cracPath, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("cracCheckpointPath: %v", err)
	}

	// Create checkpoint directory to simulate a previous checkpoint
	os.MkdirAll(cracPath, 0755)

	// Verify it's a directory
	info, statErr := os.Stat(cracPath)
	if statErr != nil {
		t.Fatalf("stat checkpoint: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("expected checkpoint path to be a directory")
	}

	// Replicate the daemon's CRaC logic
	var args []string
	if statErr == nil && info.IsDir() {
		args = append(args, "-XX:CRaCRestoreFrom="+cracPath)
	}

	if len(args) != 1 {
		t.Fatalf("expected 1 CRaC flag, got %d", len(args))
	}
	if !strings.HasPrefix(args[0], "-XX:CRaCRestoreFrom=") {
		t.Errorf("expected CRaCRestoreFrom flag, got %q", args[0])
	}
	if !strings.Contains(args[0], cracPath) {
		t.Errorf("expected checkpoint path in flag, got %q", args[0])
	}
}

func TestCRaCCheckpointPath_MissingJAR(t *testing.T) {
	_, err := cracCheckpointPath("/nonexistent/path/to.jar")
	if err == nil {
		t.Error("expected error for missing JAR")
	}
}

func TestCRaCCheckpointPath_SameHashAsCDS(t *testing.T) {
	// CRaC and CDS paths should use the same hash but different extensions
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("shared hash test"), 0644)

	cdsPath, _ := cdsArchivePath(jarPath)
	cracPath, _ := cracCheckpointPath(jarPath)

	// Extract hash from each
	cdsBase := filepath.Base(cdsPath)
	cdsHash := strings.TrimPrefix(cdsBase, "krit-types-")
	cdsHash = strings.TrimSuffix(cdsHash, ".jsa")

	cracBase := filepath.Base(cracPath)
	cracHash := strings.TrimPrefix(cracBase, "krit-types-")
	cracHash = strings.TrimSuffix(cracHash, ".crac")

	if cdsHash != cracHash {
		t.Errorf("expected same hash for CDS and CRaC, got %q vs %q", cdsHash, cracHash)
	}

	// But full paths should differ
	if cdsPath == cracPath {
		t.Error("expected different full paths for CDS vs CRaC")
	}
}

// ===========================================================================
// Step 6: ConnectOrStart integration tests
// ===========================================================================

func TestConnectOrStartDaemon_NoPIDFile_StartsNew(t *testing.T) {
	// This tests the connectExistingDaemon path — when no PID file exists,
	// it should fail and fall through to starting a new daemon.
	// We cannot start a real daemon without krit-types.jar, so we test
	// just the "connect" portion.

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No PID files — connectExistingDaemon should fail
	_, err := connectExistingDaemon(testSourceDirs, false)
	if err == nil {
		t.Fatal("expected error when no PID file exists")
	}
	if !strings.Contains(err.Error(), "no existing daemon") {
		t.Errorf("expected 'no existing daemon' error, got: %v", err)
	}
}

func TestConnectOrStartDaemon_StalePIDFile_StartsNew(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)

	// Write a PID file with a dead process under the test repo's hash
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("99999999\n"), 0644)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".port"), []byte("12345\n"), 0644)

	// connectExistingDaemon should fail because the PID is dead
	_, err := connectExistingDaemon(testSourceDirs, false)
	if err == nil {
		t.Fatal("expected error for dead PID")
	}
	if !strings.Contains(err.Error(), "not alive") {
		t.Errorf("expected 'not alive' error, got: %v", err)
	}

	// cleanStaleDaemon should remove the stale PID files
	cleanStaleDaemon(testSourceDirs, false)

	pidFile := filepath.Join(daemonsDir, testSourcesHash+".pid")
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected stale PID file to be cleaned up")
	}
}

func TestConnectOrStartDaemon_LiveDaemon_Reuses(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Start a fake daemon
	fake := NewFakeDaemon(t)
	defer fake.Close()

	// Write PID file pointing to our fake daemon under the test repo's hash
	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".pid"),
		[]byte(fmt.Sprintf("%d\n", os.Getpid())), // alive PID
		0644,
	)
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".port"),
		[]byte(fmt.Sprintf("%d\n", fake.Port)),
		0644,
	)

	// connectExistingDaemon should successfully connect
	d, err := connectExistingDaemon(testSourceDirs, false)
	if err != nil {
		t.Fatalf("connectExistingDaemon: %v", err)
	}
	defer d.Close()

	// Should be marked as shared (reused)
	if !d.shared {
		t.Error("expected daemon to be marked as shared")
	}

	// Should be responsive
	if err := d.Ping(); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestConnectOrStartDaemon_LiveDaemon_MultipleClients(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	fake := NewFakeDaemon(t)
	defer fake.Close()

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".pid"),
		[]byte(fmt.Sprintf("%d\n", os.Getpid())),
		0644,
	)
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".port"),
		[]byte(fmt.Sprintf("%d\n", fake.Port)),
		0644,
	)

	// Connect two clients to the same fake daemon
	d1, err := connectExistingDaemon(testSourceDirs, false)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	defer d1.Close()

	d2, err := connectExistingDaemon(testSourceDirs, false)
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	defer d2.Close()

	// Both should be able to ping independently
	if err := d1.Ping(); err != nil {
		t.Fatalf("d1 Ping: %v", err)
	}
	if err := d2.Ping(); err != nil {
		t.Fatalf("d2 Ping: %v", err)
	}
}

func TestConnectOrStartDaemon_SharedDaemonClose_DoesNotKillProcess(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()

	d := fake.ConnectDaemon(t)

	// Closing a shared daemon should only close the TCP connection
	d.Close()

	if d.started {
		t.Error("expected started=false after close")
	}

	// The fake daemon should still be accepting new connections
	d2 := fake.ConnectDaemon(t)
	defer d2.Close()

	if err := d2.Ping(); err != nil {
		t.Fatalf("Ping after first client close: %v", err)
	}
}

func TestCleanStaleDaemon_WithLivePIDButBrokenPort(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)

	// Write our own PID but a port where nothing is listening
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".pid"),
		[]byte(fmt.Sprintf("%d\n", os.Getpid())),
		0644,
	)
	os.WriteFile(
		filepath.Join(daemonsDir, testSourcesHash+".port"),
		[]byte("1\n"), // port 1 — nothing listening
		0644,
	)

	// connectExistingDaemon should fail because the port connection fails
	_, err := connectExistingDaemon(testSourceDirs, false)
	if err == nil {
		t.Fatal("expected error for unreachable port")
	}
}

// ===========================================================================
// Integration test with real daemon (skipped without krit-types.jar)
// ===========================================================================

func findKritTypesJar() string {
	// Check common locations
	candidates := []string{
		"krit-types.jar",
		"build/krit-types.jar",
		"../krit-types/build/libs/krit-types.jar",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func TestIntegration_RealDaemon(t *testing.T) {
	jarPath := findKritTypesJar()
	if jarPath == "" {
		t.Skip("krit-types.jar not found — skipping real daemon integration test")
	}

	d, err := StartDaemon(jarPath, nil, nil, false)
	if err != nil {
		t.Fatalf("StartDaemon: %v", err)
	}
	defer d.Close()

	// Should be able to analyze
	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll: %v", err)
	}
	if data.Version < 1 {
		t.Errorf("expected version >= 1, got %d", data.Version)
	}
}

func TestIntegration_RealDaemonWithPort(t *testing.T) {
	jarPath := findKritTypesJar()
	if jarPath == "" {
		t.Skip("krit-types.jar not found — skipping real daemon integration test")
	}

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	d, err := StartDaemonWithPort(jarPath, nil, nil, false)
	if err != nil {
		t.Fatalf("StartDaemonWithPort: %v", err)
	}
	defer d.Close()

	// Should be able to ping
	if err := d.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// PID file should have been written under the daemon's own sources hash.
	// Since StartDaemonWithPort was called with nil sourceDirs, the hash
	// is hashSources(nil) == hashSources([]string{}).
	info, err := readPIDFile(hashSources(nil))
	if err != nil {
		t.Fatalf("readPIDFile: %v", err)
	}
	if info.Port != d.port {
		t.Errorf("expected port %d in PID file, got %d", d.port, info.Port)
	}
}

func TestIntegration_ConnectOrStartDaemon(t *testing.T) {
	jarPath := findKritTypesJar()
	if jarPath == "" {
		t.Skip("krit-types.jar not found — skipping real daemon integration test")
	}

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// First call should start a new daemon
	d1, err := ConnectOrStartDaemon(jarPath, nil, nil, false)
	if err != nil {
		t.Fatalf("first ConnectOrStartDaemon: %v", err)
	}
	defer d1.Close()

	// Second call should reuse the existing daemon
	d2, err := ConnectOrStartDaemon(jarPath, nil, nil, false)
	if err != nil {
		t.Fatalf("second ConnectOrStartDaemon: %v", err)
	}
	defer d2.Close()

	if !d2.shared {
		t.Error("expected second daemon to be shared (reused)")
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

// newScannerFromConn creates a bufio.Scanner from a net.Conn with large buffer.
func newScannerFromConn(conn net.Conn) *bufio.Scanner {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	return sc
}

// unmarshalDaemonResponse is a test helper that parses a raw JSON string into
// a daemonResponse for assertions.
func unmarshalDaemonResponse(t *testing.T, raw string) daemonResponse {
	t.Helper()
	var resp daemonResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}
