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

func TestAppendExtraJVMArgsBeforeJar(t *testing.T) {
	args := []string{"-Xms1g", "-jar", "krit-types.jar", "--daemon"}
	got := appendExtraJVMArgsBeforeJar(args, []string{"-Xmx2g", "-XX:ActiveProcessorCount=6"})
	want := []string{"-Xms1g", "-Xmx2g", "-XX:ActiveProcessorCount=6", "-jar", "krit-types.jar", "--daemon"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendExtraJVMArgsBeforeJar = %#v, want %#v", got, want)
	}
}
