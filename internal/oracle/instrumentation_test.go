package oracle

import (
	"reflect"
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

func TestAppendExtraJVMArgsBeforeJar(t *testing.T) {
	args := []string{"-Xms1g", "-jar", "krit-types.jar", "--daemon"}
	got := appendExtraJVMArgsBeforeJar(args, []string{"-Xmx2g", "-XX:ActiveProcessorCount=6"})
	want := []string{"-Xms1g", "-Xmx2g", "-XX:ActiveProcessorCount=6", "-jar", "krit-types.jar", "--daemon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendExtraJVMArgsBeforeJar = %#v, want %#v", got, want)
	}
}
