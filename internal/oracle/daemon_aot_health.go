package oracle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/jvmaot"
)

// ErrOracleAOTDegraded reports that a daemon launched from a Leyden AOT
// cache answered a non-empty analysis request with no file facts and no
// errors. Sidecar validation can't catch a cache that loads but leaves the
// JVM unable to analyze, so this is the oracle's runtime self-check (the
// counterpart of checkFirWithRecovery in internal/firchecks).
var ErrOracleAOTDegraded = errors.New("oracle daemon AOT cache produced no analysis facts")

var oracleAOTWarningOnce sync.Once

// startupAOTCache returns the AOT cache a JVM launch uses, or "" when the
// launch records a profile or runs without AOT.
func startupAOTCache(args []string) string {
	for _, arg := range args {
		if path, ok := strings.CutPrefix(arg, "-XX:AOTCache="); ok {
			return path
		}
	}
	return ""
}

// checkAOTHealthLocked runs once per daemon, on the first response to a
// request that named files. Every analyzed file yields a file entry (an
// empty one when it has no declarations) and every failure is reported, so
// an empty answer with nothing reported means the JVM can't analyze. The
// poisoned cache is disabled for this process and discarded, the daemon is
// retired, and the error sends callers to their non-daemon fallback, which
// then launches without AOT. Caller must hold d.mu.
func (d *Daemon) checkAOTHealthLocked(requested int, data *Data, reported int) error {
	if d.aotCachePath == "" || requested == 0 {
		return nil
	}
	first := false
	d.aotCheckOnce.Do(func() { first = true })
	if !first || (data != nil && len(data.Files) > 0) || reported > 0 {
		return nil
	}
	cachePath := d.aotCachePath
	oracleAOTWarningOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "warning: oracle AOT cache produced no analysis facts; retrying without AOT")
	})
	jvmaot.DisableForProcess(cachePath)
	jvmaot.DiscardCache(cachePath)
	d.retireLocked()
	return fmt.Errorf("%w: %s", ErrOracleAOTDegraded, cachePath)
}

// retireLocked stops a daemon this process started and removes its registry
// entry, so no later client reuses the poisoned JVM. Caller must hold d.mu.
func (d *Daemon) retireLocked() {
	d.started = false
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_ = d.cmd.Wait()
	}
	if d.port != 0 && d.sourcesHash != "" {
		removePIDFileSlot(d.sourcesHash, d.slot)
	}
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

// nestedResultErrorCount counts the per-file errors that the legacy analyze
// envelopes (analyze, analyzeFiles) nest inside result.
func nestedResultErrorCount(result *json.RawMessage) int {
	if result == nil {
		return 0
	}
	var nested struct {
		Errors map[string]string `json:"errors"`
	}
	if json.Unmarshal(*result, &nested) != nil {
		return 0
	}
	return len(nested.Errors)
}
