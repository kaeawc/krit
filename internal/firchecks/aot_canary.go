package firchecks

import (
	"fmt"
	"os"
	"path/filepath"
)

// The AOT canary is the runtime self-check for a krit-fir checker daemon
// launched from a Leyden AOT cache (#768). Sidecar validation proves the
// cache file is the one that was published, and the registered-rules check
// proves the extension registered its checkers, but neither proves the
// checkers still report. The canary compiles a tiny built-in source whose
// finding is fixed and always expected, so a JVM that registers its rules and
// then returns zero findings for files it analyzed fails closed instead of
// silently dropping findings.
//
// The oracle daemon's equivalent is checkAOTHealthLocked in internal/oracle.

// aotCanaryRule is the checker the canary exercises. DefaultLocale is a
// built-in rule that needs only java.lang, so the canary compiles against
// any classpath the real request uses.
const aotCanaryRule = "DefaultLocale"

// aotCanaryFile is the base name krit-fir echoes back in the finding path.
const aotCanaryFile = "Canary.kt"

// aotCanarySource has exactly one DefaultLocale finding: a String.format call
// without a Locale.
const aotCanarySource = "package krit.canary\n\nfun canary(): String = String.format(\"%d\", 1)\n"

// canaryHealthy reports whether resp proves the canary finding came back.
func canaryHealthy(resp *CheckResponse) bool {
	if resp == nil {
		return false
	}
	for _, f := range resp.Findings {
		if f.Rule == aotCanaryRule && filepath.Base(f.Path) == aotCanaryFile {
			return true
		}
	}
	return false
}

// runAOTCanary checks the canary source on d and reports whether the
// expected finding came back. It sends the real request's source dirs,
// classpath, and JVM target, so the daemon reuses its session instead of
// rebuilding it. A transport error is returned as err: the daemon did not
// answer, which is a different failure than answering wrongly.
func (d *FirDaemon) runAOTCanary(sourceDirs, classpath []string, target ...string) (healthy bool, err error) {
	dir, err := os.MkdirTemp("", "krit-fir-canary-")
	if err != nil {
		return false, fmt.Errorf("create FIR canary dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, aotCanaryFile)
	if err := os.WriteFile(path, []byte(aotCanarySource), 0o600); err != nil {
		return false, fmt.Errorf("write FIR canary: %w", err)
	}
	resp, err := d.Check([]fileRef{{Path: path}}, sourceDirs, classpath, []string{aotCanaryRule}, nil, FileFacts{}, target...)
	if err != nil {
		return false, err
	}
	return canaryHealthy(resp), nil
}
