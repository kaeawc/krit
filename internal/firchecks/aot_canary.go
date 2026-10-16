package firchecks

import "path/filepath"

// The AOT canary is the runtime self-check for a krit-fir checker daemon
// launched from a Leyden AOT cache (#768). Sidecar validation proves the
// cache file is the one that was published, and the registered-rules check
// proves the extension registered its checkers, but neither proves the
// checkers still report. The daemon's `canary` request compiles a tiny
// built-in source (tools/krit-fir Canary.kt) with one known DefaultLocale
// finding in an isolated session, so a JVM that registers its rules and then
// returns zero findings fails closed instead of silently dropping findings.
// The cost is one small compile, independent of the project size.
//
// A jar that predates the request answers with an error and no findings,
// which reads as unhealthy: such a daemon is never trusted.
//
// The oracle daemon's equivalent is checkAOTHealthLocked in internal/oracle.
// Oracle-role krit-fir daemons go through the same launch path but have no
// canary of their own; one is a prerequisite for re-enabling AOT for them.

// aotCanaryRule is the checker the canary exercises; it must match
// CANARY_RULE in tools/krit-fir.
const aotCanaryRule = "DefaultLocale"

// aotCanaryFile must match CANARY_FILE in tools/krit-fir.
const aotCanaryFile = "Canary.kt"

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

// runAOTCanary reports whether the daemon returned the canary finding. A
// transport error is returned as err: the daemon did not answer, which is a
// different failure than answering wrongly.
func (d *FirDaemon) runAOTCanary() (healthy bool, err error) {
	resp, err := d.Canary()
	if err != nil {
		return false, err
	}
	return canaryHealthy(resp), nil
}
