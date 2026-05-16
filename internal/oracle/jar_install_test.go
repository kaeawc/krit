package oracle

import (
	"context"
	"strings"
	"testing"
)

func TestEnsureJar_DevBuildReturnsHelpfulError(t *testing.T) {
	isolateJarLookup(t)
	// Version stays "" → versionTag() returns "" → no release URL → error.
	_, err := EnsureJar(context.Background(), nil, false)
	if err == nil {
		t.Fatal("expected error for dev build with no jar")
	}
	msg := err.Error()
	for _, want := range []string{"krit-types.jar not found", "KRIT_TYPES_JAR", "./gradlew shadowJar"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}
}

func TestJarReleaseURL_NormalisesVersion(t *testing.T) {
	prev := Version
	t.Cleanup(func() { Version = prev })

	Version = "1.2.3"
	if got, want := jarReleaseURL(), "https://github.com/kaeawc/krit/releases/download/v1.2.3/krit-types-v1.2.3.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	Version = "v1.2.3"
	if got, want := jarReleaseURL(), "https://github.com/kaeawc/krit/releases/download/v1.2.3/krit-types-v1.2.3.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	Version = "dev"
	if got := jarReleaseURL(); got != "" {
		t.Errorf("dev build should not have a release URL, got %q", got)
	}
	Version = ""
	if got := jarReleaseURL(); got != "" {
		t.Errorf("empty version should not have a release URL, got %q", got)
	}
}
