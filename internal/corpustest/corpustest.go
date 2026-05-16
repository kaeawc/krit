// Package corpustest centralizes access to the KOTLIN_CORPUS and
// SIGNAL_ANDROID_CORPUS environment variables that point benchmarks
// and integration tests at an external source corpus. Keeping the
// var names + fallback strings in one place means contributors can
// override either via a gitignored .env (see .env.example) without
// patching test files, and reviewers don't see absolute paths baked
// into committed code.
//
// Two access patterns:
//
//   - KotlinCorpusPath() / SignalAndroidCorpusPath(): return the env
//     value when set, or a generic "/corpus/<name>" placeholder when
//     unset. Use this for synthetic benchmarks that only need a
//     plausible-looking path STRING — they don't read from disk, so
//     the placeholder is fine.
//
//   - RequireKotlinCorpus(tb) / RequireSignalAndroidCorpus(tb): when
//     the env var is set, return the path; when unset, t.Skip the
//     calling bench/test with a clear message. Use this for tests
//     that actually READ the corpus from disk.
package corpustest

import (
	"os"
	"testing"
)

// EnvKotlinCorpus is the name of the env var pointing at the Kotlin
// compiler corpus (https://github.com/JetBrains/kotlin) — Krit's
// large-Kotlin reference workload.
const EnvKotlinCorpus = "KOTLIN_CORPUS"

// EnvSignalAndroidCorpus is the name of the env var pointing at the
// Signal-Android corpus (https://github.com/signalapp/Signal-Android)
// — Krit's reference Android workload.
const EnvSignalAndroidCorpus = "SIGNAL_ANDROID_CORPUS"

// PlaceholderKotlinCorpus is the path-shape placeholder used by
// synthetic benchmarks when KOTLIN_CORPUS is unset. Looks like a
// real path so file-path-length sensitive code paths (string
// allocation, hash-key construction) measure realistically.
const PlaceholderKotlinCorpus = "/corpus/kotlin"

// PlaceholderSignalAndroidCorpus mirrors PlaceholderKotlinCorpus
// for Signal-Android.
const PlaceholderSignalAndroidCorpus = "/corpus/signal-android"

// KotlinCorpusPath returns the value of KOTLIN_CORPUS, or
// PlaceholderKotlinCorpus when the env var is unset. Triggers a
// lazy .env load on first call (see LoadDotEnv) so contributors
// don't need to source the file before invoking go test.
func KotlinCorpusPath() string {
	LoadDotEnv()
	if p := os.Getenv(EnvKotlinCorpus); p != "" {
		return p
	}
	return PlaceholderKotlinCorpus
}

// SignalAndroidCorpusPath returns the value of SIGNAL_ANDROID_CORPUS,
// or PlaceholderSignalAndroidCorpus when the env var is unset.
// Triggers a lazy .env load on first call.
func SignalAndroidCorpusPath() string {
	LoadDotEnv()
	if p := os.Getenv(EnvSignalAndroidCorpus); p != "" {
		return p
	}
	return PlaceholderSignalAndroidCorpus
}

// RequireKotlinCorpus returns the value of KOTLIN_CORPUS, or skips
// tb when the env var is unset. Use from benchmarks/tests that
// actually read from the corpus directory. Triggers a lazy .env
// load on first call.
func RequireKotlinCorpus(tb testing.TB) string {
	tb.Helper()
	LoadDotEnv()
	p := os.Getenv(EnvKotlinCorpus)
	if p == "" {
		tb.Skipf("%s not set; skipping corpus-driven test (see .env.example)", EnvKotlinCorpus)
	}
	return p
}

// RequireSignalAndroidCorpus mirrors RequireKotlinCorpus for the
// Signal-Android corpus.
func RequireSignalAndroidCorpus(tb testing.TB) string {
	tb.Helper()
	LoadDotEnv()
	p := os.Getenv(EnvSignalAndroidCorpus)
	if p == "" {
		tb.Skipf("%s not set; skipping corpus-driven test (see .env.example)", EnvSignalAndroidCorpus)
	}
	return p
}
