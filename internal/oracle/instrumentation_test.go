package oracle

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestExtraJVMArgsFromEnv(t *testing.T) {
	t.Setenv("KRIT_TYPES_EXTRA_JVM_ARGS", "  -XX:ActiveProcessorCount=6   -Xmx2g  ")

	got := extraJVMArgsFromEnv()
	want := []string{"-XX:ActiveProcessorCount=6", "-Xmx2g"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extraJVMArgsFromEnv = %#v, want %#v", got, want)
	}
}

func TestConfiguredExtraJVMArgsPrefersOptions(t *testing.T) {
	t.Setenv("KRIT_TYPES_EXTRA_JVM_ARGS", "-Xmx2g")

	got := configuredExtraJVMArgs(InvocationOptions{
		ExtraJVMArgs: []string{"-XX:ActiveProcessorCount=4"},
	})
	want := []string{"-XX:ActiveProcessorCount=4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredExtraJVMArgs = %#v, want %#v", got, want)
	}
}

func TestActiveProcessorCountForKritTypesShard(t *testing.T) {
	if got := activeProcessorCountForKritTypesShard(1); got != 0 {
		t.Fatalf("shards=1: got %d, want 0 (no cap)", got)
	}
	if got := activeProcessorCountForKritTypesShard(0); got != 0 {
		t.Fatalf("shards=0: got %d, want 0", got)
	}

	cpus := runtime.GOMAXPROCS(0)
	if cpus <= 0 {
		cpus = runtime.NumCPU()
	}

	got2 := activeProcessorCountForKritTypesShard(2)
	if got2 < 1 {
		t.Fatalf("shards=2: got %d, want >= 1", got2)
	}
	// perShard = ceil(cpus/2) - 1 (min 1)
	perShard := (cpus + 1) / 2
	if perShard > 1 {
		perShard--
	}
	if perShard < 1 {
		perShard = 1
	}
	if got2 != perShard {
		t.Fatalf("shards=2, cpus=%d: got %d, want %d", cpus, got2, perShard)
	}

	// Very large shard count: each shard gets at least 1 processor.
	got := activeProcessorCountForKritTypesShard(1000)
	if got < 1 {
		t.Fatalf("shards=1000: got %d, want >= 1", got)
	}
}

func TestJvmArgsForKritTypesShard(t *testing.T) {
	if got := jvmArgsForKritTypesShard(1); got != nil {
		t.Fatalf("shards=1: got %v, want nil", got)
	}

	got := jvmArgsForKritTypesShard(2)
	if len(got) != 1 {
		t.Fatalf("shards=2: got %v, want 1 arg", got)
	}
	if !strings.HasPrefix(got[0], "-XX:ActiveProcessorCount=") {
		t.Fatalf("shards=2: arg = %q, want -XX:ActiveProcessorCount=...", got[0])
	}
}

func TestAdaptiveShardJVMArgs_NoCap_WhenShardsOne(t *testing.T) {
	t.Setenv("KRIT_TYPES_EXTRA_JVM_ARGS", "-Xmx2g")
	got := adaptiveShardJVMArgs(1, InvocationOptions{})
	want := []string{"-Xmx2g"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shards=1: got %v, want %v", got, want)
	}
}

func TestAdaptiveShardJVMArgs_EnvAppendsAfterPolicy(t *testing.T) {
	t.Setenv("KRIT_TYPES_EXTRA_JVM_ARGS", "-Xmx2g")
	got := adaptiveShardJVMArgs(2, InvocationOptions{})
	if len(got) < 2 {
		t.Fatalf("shards=2 with env: got %v, want >= 2 args", got)
	}
	if !strings.HasPrefix(got[0], "-XX:ActiveProcessorCount=") {
		t.Fatalf("first arg should be ActiveProcessorCount, got %q", got[0])
	}
	if got[len(got)-1] != "-Xmx2g" {
		t.Fatalf("last arg should be -Xmx2g (env override), got %q", got[len(got)-1])
	}
}

func TestAdaptiveShardJVMArgs_OptsOverrideEnv(t *testing.T) {
	t.Setenv("KRIT_TYPES_EXTRA_JVM_ARGS", "-Xmx8g")
	got := adaptiveShardJVMArgs(2, InvocationOptions{ExtraJVMArgs: []string{"-Xmx4g"}})
	// opts.ExtraJVMArgs takes precedence over env, both appended after adaptive
	last := got[len(got)-1]
	if last != "-Xmx4g" {
		t.Fatalf("opts ExtraJVMArgs should take precedence over env, got last=%q in %v", last, got)
	}
	for _, a := range got {
		if a == "-Xmx8g" {
			t.Fatalf("env arg should be suppressed when opts.ExtraJVMArgs is set, got %v", got)
		}
	}
}

func TestConfiguredKritTypesParallelFilesDefault(t *testing.T) {
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	if got := configuredKritTypesParallelFiles(); got != defaultKritTypesParallelFiles {
		t.Fatalf("default parallel files = %d, want %d", got, defaultKritTypesParallelFiles)
	}
	want := []string{"--experimental-parallel-files", "4"}
	if got := experimentalParallelFilesArg(); !reflect.DeepEqual(got, want) {
		t.Fatalf("experimentalParallelFilesArg = %#v, want %#v", got, want)
	}
}

func TestConfiguredKritTypesParallelFilesEnvOverride(t *testing.T) {
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "2")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	if got := configuredKritTypesParallelFiles(); got != 2 {
		t.Fatalf("parallel files override = %d, want 2", got)
	}
	want := []string{"--experimental-parallel-files", "2"}
	if got := experimentalParallelFilesArg(); !reflect.DeepEqual(got, want) {
		t.Fatalf("experimentalParallelFilesArg = %#v, want %#v", got, want)
	}
}

func TestConfiguredKritTypesParallelFilesCanDisable(t *testing.T) {
	for _, value := range []string{"0", "1", "auto"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("KRIT_TYPES_PARALLEL_FILES", value)
			t.Setenv("KRIT_TYPES_SHARDS", "")

			if got := configuredKritTypesParallelFiles(); got != 0 {
				t.Fatalf("disabled parallel files = %d, want 0", got)
			}
			if got := experimentalParallelFilesArg(); got != nil {
				t.Fatalf("experimentalParallelFilesArg = %#v, want nil", got)
			}
		})
	}
}

func TestConfiguredKritTypesParallelFilesShardGuard(t *testing.T) {
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "4")

	if got := configuredKritTypesParallelFiles(); got != 0 {
		t.Fatalf("parallel files with explicit shards = %d, want 0", got)
	}
	if got := experimentalParallelFilesArg(); got != nil {
		t.Fatalf("experimentalParallelFilesArg = %#v, want nil", got)
	}

	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "3")
	if got := configuredKritTypesParallelFiles(); got != 3 {
		t.Fatalf("explicit parallel files with shards = %d, want 3", got)
	}
}

func TestAppendExtraJVMArgsBeforeJar(t *testing.T) {
	args := []string{"-Xms1g", "-jar", "krit-types.jar", "--daemon"}
	got := appendExtraJVMArgsBeforeJar(args, []string{"-Xmx2g", "-XX:ActiveProcessorCount=6"})
	want := []string{"-Xms1g", "-Xmx2g", "-XX:ActiveProcessorCount=6", "-jar", "krit-types.jar", "--daemon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendExtraJVMArgsBeforeJar = %#v, want %#v", got, want)
	}
}
