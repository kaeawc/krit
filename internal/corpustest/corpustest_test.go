package corpustest

import (
	"testing"
)

func TestKotlinCorpusPath_FallbackWhenUnset(t *testing.T) {
	t.Setenv(EnvKotlinCorpus, "")
	if got, want := KotlinCorpusPath(), PlaceholderKotlinCorpus; got != want {
		t.Errorf("KotlinCorpusPath() = %q, want %q", got, want)
	}
}

func TestKotlinCorpusPath_HonorsEnv(t *testing.T) {
	t.Setenv(EnvKotlinCorpus, "/tmp/elsewhere/kotlin")
	if got, want := KotlinCorpusPath(), "/tmp/elsewhere/kotlin"; got != want {
		t.Errorf("KotlinCorpusPath() = %q, want %q", got, want)
	}
}

func TestSignalAndroidCorpusPath_FallbackWhenUnset(t *testing.T) {
	t.Setenv(EnvSignalAndroidCorpus, "")
	if got, want := SignalAndroidCorpusPath(), PlaceholderSignalAndroidCorpus; got != want {
		t.Errorf("SignalAndroidCorpusPath() = %q, want %q", got, want)
	}
}

func TestSignalAndroidCorpusPath_HonorsEnv(t *testing.T) {
	t.Setenv(EnvSignalAndroidCorpus, "/tmp/elsewhere/signal")
	if got, want := SignalAndroidCorpusPath(), "/tmp/elsewhere/signal"; got != want {
		t.Errorf("SignalAndroidCorpusPath() = %q, want %q", got, want)
	}
}

// TestRequireKotlinCorpus_SkipsWhenUnset confirms a Require* call
// fires t.Skip rather than returning an empty path — corpus-driven
// benches must skip cleanly on machines without the checkout.
func TestRequireKotlinCorpus_SkipsWhenUnset(t *testing.T) {
	t.Setenv(EnvKotlinCorpus, "")
	skipped := runSubtestAndReportSkip(t, func(tb testing.TB) {
		_ = RequireKotlinCorpus(tb)
	})
	if !skipped {
		t.Error("RequireKotlinCorpus did not skip when env unset")
	}
}

func TestRequireSignalAndroidCorpus_SkipsWhenUnset(t *testing.T) {
	t.Setenv(EnvSignalAndroidCorpus, "")
	skipped := runSubtestAndReportSkip(t, func(tb testing.TB) {
		_ = RequireSignalAndroidCorpus(tb)
	})
	if !skipped {
		t.Error("RequireSignalAndroidCorpus did not skip when env unset")
	}
}

func TestRequireKotlinCorpus_ReturnsPath(t *testing.T) {
	t.Setenv(EnvKotlinCorpus, "/tmp/kotlin")
	got := RequireKotlinCorpus(t)
	if got != "/tmp/kotlin" {
		t.Errorf("RequireKotlinCorpus() = %q, want %q", got, "/tmp/kotlin")
	}
}

// runSubtestAndReportSkip runs fn inside a subtest, returning true
// when the subtest skipped. Lets the Require* tests observe the
// Skip outcome without aborting the parent. The defer captures the
// Skipped() result via runtime.Goexit's deferred-call guarantee —
// reading after fn() would never execute on skip.
func runSubtestAndReportSkip(t *testing.T, fn func(tb testing.TB)) bool {
	t.Helper()
	skipped := false
	t.Run("inner", func(sub *testing.T) {
		defer func() { skipped = sub.Skipped() }()
		fn(sub)
	})
	return skipped
}
