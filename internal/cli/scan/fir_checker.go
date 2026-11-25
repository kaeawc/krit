package scan

import (
	"context"
	"fmt"
	"io"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/modshard"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

// firCheckerOpts groups every flag and runtime input runFIRCheckerPass
// needs. The pass itself lives in firchecks so the daemon's analyze-project
// verb (internal/cli/serve) runs the same code.
type firCheckerOpts = firchecks.PassOptions

// runFIRCheckerPass runs the FIR checkers and applies the FIR-authoritative
// verdict to base. No-op when opts.Enabled is false.
func runFIRCheckerPass(opts firCheckerOpts, base []scanner.Finding) []scanner.Finding {
	return firchecks.RunPass(opts, base)
}

// SourceSetIsolator returns the --fir pass's recheck of projects whose merged
// compile reports a duplicate declaration between source sets, or nil when
// KRIT_MODULE_SHARDS=merged turns it off or the scan reads no Gradle model.
// modelDir and modelDisabled are the scan's --gradle-model and
// --no-gradle-model. Verbose progress goes to verboseOut.
func SourceSetIsolator(paths []string, modelDir string, modelDisabled, noCache bool, verboseOut io.Writer) firchecks.SourceSetIsolator {
	var logf func(string, ...any)
	if verboseOut != nil {
		logf = func(format string, args ...any) { fmt.Fprintf(verboseOut, format, args...) }
	}
	return modshard.Isolator(paths, modelDir, modelDisabled, noCache, logf)
}

// SettingsSplitter returns the --fir pass's recheck of the source sets whose
// Kotlin compiler settings differ from the merged session's, one session per
// settings group (#1577), or nil when KRIT_MODULE_SHARDS=merged turns it off
// or the scan reads no Gradle model. Arguments are those of SourceSetIsolator.
func SettingsSplitter(paths []string, modelDir string, modelDisabled, noCache bool, verboseOut io.Writer) firchecks.SettingsSplitter {
	var logf func(string, ...any)
	if verboseOut != nil {
		logf = func(format string, args ...any) { fmt.Fprintf(verboseOut, format, args...) }
	}
	return modshard.SettingsSplitter(paths, modelDir, modelDisabled, noCache, logf)
}

// FIRCompileContext returns the compile context the --fir pass checks
// against: the JVM-scoped source roots the oracle compiles (non-JVM KMP
// source sets dropped) and the configured oracle classpath, so the checkers
// resolve cross-file and library references the way the oracle does.
func FIRCompileContext(paths []string, cfg *config.Config) (sourceDirs, classpath []string) {
	return oracle.FilterFIRSourceDirs(oracle.FindSourceDirs(paths)), resolveOracleClasspath(cfg)
}

// NewFIRChecker builds the production checker for a scan of paths.
func NewFIRChecker(paths []string, cfg *config.Config, useDaemon, verbose bool) *firchecks.ProductionFirChecker {
	sourceDirs, classpath := FIRCompileContext(paths, cfg)
	// Tagged releases download a missing krit-fir jar. On failure JarPath
	// stays "" and the checker reports the missing jar itself.
	jarPath, _ := oracle.EnsureBackendJar(context.Background(), oracle.BackendFIR, paths, verbose)
	return &firchecks.ProductionFirChecker{
		JarPath:    jarPath,
		SourceDirs: sourceDirs,
		Classpath:  classpath,
		RepoDir:    oracle.FindRepoDir(paths),
		UseDaemon:  useDaemon,
		Verbose:    verbose,
	}
}
