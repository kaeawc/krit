package scan

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// NoticeFIRPreflight is a local-only, bounded approximation for informational
// server notices. CLI scans continue to use PreflightFIR, including jar install.
func NoticeFIRPreflight(ctx context.Context, paths []string, cfg *config.Config) error {
	java, err := oracle.JavaPath()
	if err != nil {
		return fmt.Errorf("FIR requires Java 21 or newer: %w", err)
	}
	out, err := exec.CommandContext(ctx, java, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("FIR requires Java 21 or newer: %w", err)
	}
	version := string(out)
	start := strings.IndexByte(version, '"')
	if start < 0 {
		return fmt.Errorf("FIR requires Java 21 or newer: Java version unknown")
	}
	end := strings.IndexByte(version[start+1:], '"')
	if end < 0 {
		return fmt.Errorf("FIR requires Java 21 or newer: Java version unknown")
	}
	major, _ := strconv.Atoi(strings.SplitN(strings.SplitN(version[start+1:start+1+end], ".", 2)[0], "-", 2)[0])
	if major < 21 {
		return fmt.Errorf("FIR requires Java 21 or newer (found version %d)", major)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if firchecks.FindFirJar(paths) == "" {
		return fmt.Errorf("FIR requires krit-fir.jar: jar not found locally")
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		if gradlemodel.Discover(path) != "" {
			return nil
		}
	}
	if (cfg != nil && cfg.HasTopLevelOption("oracle", "classpath")) || len(dedupePreservingOrder(splitEnvClasspath())) > 0 {
		return nil
	}
	return fmt.Errorf("FIR requires a declared classpath source: a Gradle model, oracle.classpath, or CLASSPATH")
}
