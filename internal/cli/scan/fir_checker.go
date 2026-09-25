package scan

import (
	"context"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
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

// FIRCompileContext returns the compile context the --fir pass checks
// against: the JVM-scoped source roots the oracle compiles (non-JVM KMP
// source sets dropped) and the configured oracle classpath, so the checkers
// resolve cross-file and library references the way the oracle does.
func FIRCompileContext(paths []string, cfg *config.Config) (sourceDirs, classpath []string) {
	return oracle.FindSourceDirs(paths), resolveOracleClasspath(cfg)
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
