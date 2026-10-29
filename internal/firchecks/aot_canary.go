package firchecks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/jvmaot"
	"github.com/kaeawc/krit/internal/oracle"
)

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
// Oracle-role krit-fir daemons (the --daemon --oracle-backend=fir path,
// ConnectOrStartFirDaemon) share the launch path and get the same canary,
// run as soon as an AOT launch is up (verifyOracleRoleAOT), with one
// addition: the oracle collectors are checkers of the same krit-fir
// extension, so the checker finding already proves that extension's
// checkers dispatch, but they also gate on static collector state that the
// rule checkers do not use. The oracle-role canary therefore also asks for
// the oracle facts of the canary source (package and at least one resolved
// expression). A daemon that fails either part, or does not answer, is closed
// and replaced by one launched without AOT before it serves any request.
//
// The krit-fir oracle daemon's equivalent is checkAOTHealthLocked in
// internal/oracle.

// aotCanaryRule is the checker the canary exercises; it must match
// CANARY_RULE in tools/krit-fir.
const aotCanaryRule = "DefaultLocale"

// aotCanaryFile must match CANARY_FILE in tools/krit-fir.
const aotCanaryFile = "Canary.kt"

// aotCanaryPackage must match CANARY_PACKAGE in tools/krit-fir.
const aotCanaryPackage = "krit.canary"

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

// oracleCanaryFacts is the `oracleCanary` part of an oracle-role canary
// response: the oracle facts krit-fir collected for the canary source.
type oracleCanaryFacts struct {
	PackageName string `json:"packageName"`
	Expressions int    `json:"expressions"`
	Error       string `json:"error,omitempty"`
}

// oracleCanaryHealthy reports whether facts prove the oracle collectors ran.
// A jar that predates the oracle part sends none, which reads as unhealthy.
func oracleCanaryHealthy(facts *oracleCanaryFacts) bool {
	return facts != nil && facts.Error == "" && facts.PackageName == aotCanaryPackage && facts.Expressions > 0
}

// runOracleAOTCanary is runAOTCanary for an oracle-role daemon: the checker
// finding and the oracle facts must both come back.
func (d *FirDaemon) runOracleAOTCanary() (healthy bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return false, fmt.Errorf("fir daemon not started")
	}
	id := d.nextID
	d.nextID++
	data, err := json.Marshal(struct {
		ID      int64  `json:"id"`
		Command string `json:"command"`
		Oracle  bool   `json:"oracle"`
	}{id, "canary", true})
	if err != nil {
		return false, err
	}
	line, err := d.exchangeLocked(data)
	if err != nil {
		return false, err
	}
	resp, err := decodeCheckResponse([]byte(line), id, oracle.PathSpelling{})
	if err != nil {
		return false, err
	}
	var extra struct {
		OracleCanary *oracleCanaryFacts `json:"oracleCanary"`
	}
	_ = json.Unmarshal([]byte(line), &extra)
	return canaryHealthy(resp) && oracleCanaryHealthy(extra.OracleCanary), nil
}

// verifyOracleRoleAOT checks a freshly launched oracle-role daemon d that runs
// from an AOT cache, before the caller sends it any request. d is not
// registered yet (publishFirDaemon keeps AOT launches private): a passing
// canary publishes it through markAOTVerified. Otherwise d is closed, the
// cache is disabled for this process (and discarded when the daemon answered
// wrongly, as opposed to not answering), and the daemon that serves the
// caller is launched without AOT and registered as usual.
func verifyOracleRoleAOT(d *FirDaemon, jarPath, srcHash, role string, compile CompileTarget, verbose bool) (*FirDaemon, error) {
	healthy, err := d.runOracleAOTCanary()
	if err == nil && healthy {
		if verbose {
			reporter().Verbosef("verbose: FIR oracle AOT canary passed for %s\n", d.aotCachePath)
		}
		d.markAOTVerified()
		return d, nil
	}
	cachePath := d.aotCachePath
	if verbose {
		if err != nil {
			reporter().Verbosef("verbose: FIR oracle AOT canary failed (%v); retrying without AOT\n", err)
		} else {
			reporter().Verbosef("verbose: FIR oracle AOT canary facts missing; discarding %s and retrying without AOT\n", cachePath)
		}
	}
	aotWarningOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "warning: FIR AOT cache failed its canary; retrying without AOT")
	})
	_ = d.Close()
	jvmaot.DisableForProcess(cachePath)
	if err == nil {
		jvmaot.DiscardCache(cachePath)
	}
	retry, startErr := startFirWithoutAOT(jarPath, verbose, compile)
	if startErr != nil {
		return nil, fmt.Errorf("start persistent fir daemon without AOT: %w", startErr)
	}
	retry.sourcesHash = srcHash
	retry.role = role
	retry.target = compile
	if err := publishFirDaemon(retry, retry.cmd.Process.Pid, srcHash, verbose); err != nil {
		_ = retry.conn.Close()
		killAndReap(retry.cmd)
		return nil, fmt.Errorf("write fir PID file: %w", err)
	}
	return retry, nil
}
