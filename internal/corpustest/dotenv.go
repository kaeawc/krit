package corpustest

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
)

// dotEnvFile is the filename corpustest walks up the filesystem to
// locate. Matches the project convention documented in .env.example.
const dotEnvFile = ".env"

// loadOnce gates the lazy .env load. Multiple test packages calling
// any KotlinCorpusPath/SignalAndroidCorpusPath/Require* helper share
// one parse — cheap enough to inline at every call site and avoids
// requiring contributors to wire a TestMain into each test binary.
var loadOnce sync.Once

// LoadDotEnv walks up the filesystem from the current working
// directory looking for a .env file, parses its KEY=VALUE entries,
// and sets every entry that isn't already in the process
// environment. Exported (shell-level) vars always win — the dotenv
// convention.
//
// Idempotent: only the first call does work; subsequent calls are
// no-ops. Safe to call from any goroutine. Errors (no .env found,
// permission denied, malformed file) are silently ignored — a
// missing or unreadable .env should not break tests that already
// have their env exported.
//
// Callers that want explicit control over WHEN .env loads (e.g.
// before reading their own non-corpus env vars) can call this from
// a TestMain. Otherwise it runs lazily on first KotlinCorpusPath /
// SignalAndroidCorpusPath / Require* call.
func LoadDotEnv() {
	loadOnce.Do(loadDotEnvOnce)
}

func loadDotEnvOnce() {
	path, ok := locateDotEnv()
	if !ok {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	applyDotEnv(data)
}

// locateDotEnv walks parent directories from CWD until it finds a
// .env file. Stops at the filesystem root. Returns (path, true) on
// hit, ("", false) on miss.
func locateDotEnv() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(cwd, dotEnvFile)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", false
		}
		cwd = parent
	}
}

// applyDotEnv parses data as a sequence of KEY=VALUE lines and sets
// every key that isn't already in the process environment. Lines
// starting with `#` and blank lines are ignored. Surrounding single
// or double quotes on VALUE are stripped. Malformed lines (no `=`,
// empty key) are skipped silently — matches conventional dotenv
// permissive behavior.
func applyDotEnv(data []byte) {
	for _, raw := range bytes.Split(data, []byte("\n")) {
		key, value, ok := parseDotEnvLine(raw)
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(key); set {
			// Shell-exported wins; don't clobber.
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// parseDotEnvLine splits a single line into (key, value). Returns
// ok=false for blank lines, comment lines, and malformed entries.
// Exported so the parser can be exercised independently of disk I/O
// in tests.
func parseDotEnvLine(line []byte) (key, value string, ok bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] == '#' {
		return "", "", false
	}
	eq := bytes.IndexByte(line, '=')
	if eq <= 0 {
		// No `=` or `=` at index 0 (empty key) — malformed.
		return "", "", false
	}
	k := bytes.TrimSpace(line[:eq])
	v := bytes.TrimSpace(line[eq+1:])
	if len(k) == 0 {
		return "", "", false
	}
	if n := len(v); n >= 2 {
		first, last := v[0], v[n-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			v = v[1 : n-1]
		}
	}
	return string(k), string(v), true
}
