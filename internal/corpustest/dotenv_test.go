package corpustest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvLine_BasicKeyValue(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte("FOO=bar"))
	if !ok || k != "FOO" || v != "bar" {
		t.Errorf("got (%q, %q, %v), want (FOO, bar, true)", k, v, ok)
	}
}

func TestParseDotEnvLine_TrimsSurroundingWhitespace(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte("  FOO  =  bar  "))
	if !ok || k != "FOO" || v != "bar" {
		t.Errorf("got (%q, %q, %v), want (FOO, bar, true)", k, v, ok)
	}
}

func TestParseDotEnvLine_StripsDoubleQuotes(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte(`FOO="bar baz"`))
	if !ok || k != "FOO" || v != "bar baz" {
		t.Errorf("got (%q, %q, %v), want (FOO, 'bar baz', true)", k, v, ok)
	}
}

func TestParseDotEnvLine_StripsSingleQuotes(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte(`FOO='bar baz'`))
	if !ok || k != "FOO" || v != "bar baz" {
		t.Errorf("got (%q, %q, %v), want (FOO, 'bar baz', true)", k, v, ok)
	}
}

func TestParseDotEnvLine_PreservesUnmatchedQuote(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte(`FOO="bar`))
	if !ok || k != "FOO" || v != `"bar` {
		t.Errorf("got (%q, %q, %v), want (FOO, %q, true)", k, v, ok, `"bar`)
	}
}

func TestParseDotEnvLine_EmptyValueAllowed(t *testing.T) {
	k, v, ok := parseDotEnvLine([]byte("FOO="))
	if !ok || k != "FOO" || v != "" {
		t.Errorf("got (%q, %q, %v), want (FOO, '', true)", k, v, ok)
	}
}

func TestParseDotEnvLine_RejectsBlankLine(t *testing.T) {
	if _, _, ok := parseDotEnvLine([]byte("")); ok {
		t.Error("blank line must not parse")
	}
	if _, _, ok := parseDotEnvLine([]byte("   ")); ok {
		t.Error("whitespace-only line must not parse")
	}
}

func TestParseDotEnvLine_RejectsCommentLine(t *testing.T) {
	if _, _, ok := parseDotEnvLine([]byte("# this is a comment")); ok {
		t.Error("# comment must not parse")
	}
	if _, _, ok := parseDotEnvLine([]byte("  # indented comment")); ok {
		t.Error("indented # comment must not parse")
	}
}

func TestParseDotEnvLine_RejectsMissingEquals(t *testing.T) {
	if _, _, ok := parseDotEnvLine([]byte("NO_EQUALS_HERE")); ok {
		t.Error("line without `=` must not parse")
	}
}

func TestParseDotEnvLine_RejectsEmptyKey(t *testing.T) {
	if _, _, ok := parseDotEnvLine([]byte("=somevalue")); ok {
		t.Error("empty key must not parse")
	}
}

func TestApplyDotEnv_SetsUnsetKeys(t *testing.T) {
	const key = "CORPUSTEST_DOTENV_UNSET"
	if _, set := os.LookupEnv(key); set {
		t.Fatalf("test prerequisite: %s must not be set", key)
	}
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	applyDotEnv([]byte(key + "=loaded\n"))

	if got := os.Getenv(key); got != "loaded" {
		t.Errorf("Getenv(%s) = %q, want %q", key, got, "loaded")
	}
}

func TestApplyDotEnv_DoesNotOverrideExistingKey(t *testing.T) {
	const key = "CORPUSTEST_DOTENV_PRESET"
	t.Setenv(key, "shell-wins")

	applyDotEnv([]byte(key + "=dotenv-loses\n"))

	if got := os.Getenv(key); got != "shell-wins" {
		t.Errorf("Getenv(%s) = %q; exported value must win over dotenv", key, got)
	}
}

func TestApplyDotEnv_DoesNotOverrideEmptyExportedKey(t *testing.T) {
	// Subtle case: t.Setenv(key, "") sets the var to the empty
	// string. LookupEnv returns ok=true. The dotenv loader must
	// treat this as "set, don't clobber" — matches conventional
	// dotenv semantics where an explicit shell export, even of an
	// empty value, beats the file.
	const key = "CORPUSTEST_DOTENV_EMPTY_EXPORTED"
	t.Setenv(key, "")

	applyDotEnv([]byte(key + "=should-not-apply\n"))

	if got := os.Getenv(key); got != "" {
		t.Errorf("Getenv(%s) = %q; explicit empty export must beat dotenv", key, got)
	}
}

func TestApplyDotEnv_HandlesMultipleLines(t *testing.T) {
	const k1 = "CORPUSTEST_DOTENV_MULTI_A"
	const k2 = "CORPUSTEST_DOTENV_MULTI_B"
	t.Cleanup(func() { _ = os.Unsetenv(k1); _ = os.Unsetenv(k2) })

	applyDotEnv([]byte(
		"# header comment\n" +
			"\n" +
			k1 + "=first\n" +
			k2 + "=second\n" +
			"malformed-line-no-equals\n",
	))

	if got := os.Getenv(k1); got != "first" {
		t.Errorf("Getenv(%s) = %q, want first", k1, got)
	}
	if got := os.Getenv(k2); got != "second" {
		t.Errorf("Getenv(%s) = %q, want second", k2, got)
	}
}

// TestLocateDotEnv_FindsInCWD writes a temporary .env in a chdir'd
// directory and confirms locateDotEnv discovers it. Validates the
// directory-walk machinery without depending on the layout of the
// surrounding test environment.
func TestLocateDotEnv_FindsInCWD(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, dotEnvFile)
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("write tmp .env: %v", err)
	}
	t.Chdir(dir)

	got, ok := locateDotEnv()
	if !ok {
		t.Fatal("locateDotEnv returned ok=false for a .env in CWD")
	}
	if got != envPath {
		t.Errorf("locateDotEnv = %q, want %q", got, envPath)
	}
}

// TestLocateDotEnv_WalksUp confirms the parent-directory walk: drop
// a .env in tmp/, chdir into tmp/sub/, expect locateDotEnv to find
// the parent's .env.
func TestLocateDotEnv_WalksUp(t *testing.T) {
	parent := t.TempDir()
	sub := filepath.Join(parent, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	envPath := filepath.Join(parent, dotEnvFile)
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("write tmp .env: %v", err)
	}
	t.Chdir(sub)

	got, ok := locateDotEnv()
	if !ok {
		t.Fatal("locateDotEnv returned ok=false; walk-up failed")
	}
	if got != envPath {
		t.Errorf("locateDotEnv = %q, want %q", got, envPath)
	}
}
