package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_ProfileDispatchSurfacesTimings drives the
// --profile-dispatch wire path: the CLI sets ProfileDispatch=true on
// AnalyzeProjectArgs and the daemon must emit the per-file timing
// fan-out in DispatchProfile so the CLI can render the same stderr
// distribution table as the in-process path.
func TestAnalyzeProject_ProfileDispatchSurfacesTimings(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Profile.kt", "package demo\n\nclass Profile {\n    fun a() {}\n    fun b() {}\n}\n")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{ProfileDispatch: true}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}

	if got.DispatchProfile == nil {
		t.Fatalf("expected non-nil DispatchProfile when ProfileDispatch=true; stats=%+v", got.Stats)
	}
	if got.DispatchProfile.Workers <= 0 {
		t.Errorf("DispatchProfile.Workers = %d, want > 0", got.DispatchProfile.Workers)
	}
	if len(got.DispatchProfile.Timings) == 0 {
		t.Errorf("DispatchProfile.Timings is empty; want at least one entry")
	}
	for i, ft := range got.DispatchProfile.Timings {
		if ft.Path == "" {
			t.Errorf("DispatchProfile.Timings[%d].Path is empty", i)
		}
		if ft.Size <= 0 {
			t.Errorf("DispatchProfile.Timings[%d].Size = %d, want > 0", i, ft.Size)
		}
	}
}

// TestAnalyzeProject_ProfileDispatchOmittedByDefault pins the
// fast-path-preserving contract: when ProfileDispatch is false the
// daemon must not emit a dispatch_profile field, so the response
// envelope stays in the {findings,stats} shape the
// ScanAnalyzeProjectResponse fast path keys on.
func TestAnalyzeProject_ProfileDispatchOmittedByDefault(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Plain.kt", "package demo\n\nclass Plain\n")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.DispatchProfile != nil {
		t.Errorf("DispatchProfile should be nil without --profile-dispatch; got %+v", got.DispatchProfile)
	}
}

// TestAnalyzeProject_CPUProfileWritesFile drives the --cpuprofile
// wire path: the CLI sends an absolute path; the daemon wraps the
// analyze call in pprof.StartCPUProfile and writes the profile to
// that path on its own filesystem. ProfileWarnings must stay empty
// on the happy path.
func TestAnalyzeProject_CPUProfileWritesFile(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Cpu.kt", "package demo\n\nclass Cpu\n")
	cpuPath := filepath.Join(t.TempDir(), "cpu.pprof")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{CPUProfilePath: cpuPath}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Stats.ProfileWarnings) != 0 {
		t.Errorf("expected empty ProfileWarnings on happy path; got %v", got.Stats.ProfileWarnings)
	}
	info, err := os.Stat(cpuPath)
	if err != nil {
		t.Fatalf("stat cpu profile: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("cpu profile is empty; want non-zero bytes")
	}
}

// TestAnalyzeProject_MemProfileWritesFile mirrors the CPU test for
// the heap profile path.
func TestAnalyzeProject_MemProfileWritesFile(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Mem.kt", "package demo\n\nclass Mem\n")
	memPath := filepath.Join(t.TempDir(), "mem.pprof")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{MemProfilePath: memPath}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Stats.ProfileWarnings) != 0 {
		t.Errorf("expected empty ProfileWarnings on happy path; got %v", got.Stats.ProfileWarnings)
	}
	info, err := os.Stat(memPath)
	if err != nil {
		t.Fatalf("stat mem profile: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("mem profile is empty; want non-zero bytes")
	}
}

// TestAnalyzeProject_CPUProfileBadPathSurfacesWarning confirms the
// soft-failure contract: a profile path the daemon can't create
// (e.g. nonexistent parent directory) must populate
// AnalyzeProjectStats.ProfileWarnings and let the verb succeed —
// diagnostic capture failures must never abort the scan result.
func TestAnalyzeProject_CPUProfileBadPathSurfacesWarning(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "BadCpu.kt", "package demo\n\nclass BadCpu\n")
	badPath := "/nonexistent_dir_for_cpu_profile_test_xyz/cpu.pprof"

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{CPUProfilePath: badPath}, &got); err != nil {
		t.Fatalf("call (verb must succeed despite bad profile path): %v", err)
	}
	if len(got.Stats.ProfileWarnings) == 0 {
		t.Errorf("expected ProfileWarnings to be populated for bad cpu profile path")
	}
}
