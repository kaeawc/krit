package scan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kaeawc/krit/internal/gradlemodel"

	"github.com/kaeawc/krit/internal/config"
)

// resolveOracleClasspath assembles the user-configured classpath
// the FIR daemon's `analyze` RPC receives. Order of precedence:
//
//  1. `oracle.classpath` from krit.yml (after any model entries)
//  2. The `CLASSPATH` env var, split on the platform path separator
//
// Empty result is fine: the daemon falls back to source-tree
// dependency discovery — same shape the KAA backend uses today, so
// projects without an explicit classpath still analyze.
//
// Path validity is not checked here. The Kotlin side opens what we
// hand it; missing entries surface as resolution errors inside the
// daemon's analyze response, which is the right layer for that
// signal.
func resolveOracleClasspath(cfg *config.Config) []string {
	var out []string
	if cfg != nil {
		out = append(out, cfg.Oracle().Classpath...)
	}
	out = append(out, splitEnvClasspath()...)
	return dedupePreservingOrder(out)
}

func splitEnvClasspath() []string {
	v := os.Getenv("CLASSPATH")
	if v == "" {
		return nil
	}
	return filepath.SplitList(v)
}

// dedupePreservingOrder drops empty strings and duplicate entries
// while keeping the first occurrence's position. Stable order makes
// the wire payload deterministic for snapshot tests.
func dedupePreservingOrder(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// loadGradleClasspath loads an explicit or discovered model. An explicit
// directory failure is fatal; discovery remains best effort.
func loadGradleClasspath(paths []string, explicit string, disabled, verbose bool, out io.Writer) ([]string, error) {
	var dirs []string
	if explicit != "" {
		dirs = append(dirs, explicit)
	} else if !disabled {
		if len(paths) == 0 {
			paths = []string{"."}
		}
		seen := map[string]bool{}
		for _, path := range paths {
			if dir := gradlemodel.Discover(path); dir != "" && !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	var bootEntries, compileEntries, generatedEntries []string
	for _, dir := range dirs {
		model, warnings, err := gradlemodel.Load(dir)
		if err != nil {
			if explicit != "" {
				return nil, err
			}
			fmt.Fprintf(out, "warning: gradle model: %v\n", err)
			continue
		}
		cp, missing := model.Classpath()
		bootEntries = append(bootEntries, model.BootClasspath()...)
		compileEntries = append(compileEntries, model.CompileClasspath()...)
		generatedEntries = append(generatedEntries, model.GeneratedClasspath()...)
		if verbose {
			fmt.Fprintf(out, "gradle model: %s (%d projects, %d classpath entries, %d missing dropped)\n", dir, len(model.Projects), len(cp), missing)
		}
		for _, warning := range warnings {
			fmt.Fprintf(out, "warning: gradle model: %s\n", warning)
		}
	}
	entries := append(append(bootEntries, compileEntries...), generatedEntries...)
	return dedupePreservingOrder(entries), nil
}

// effectiveOracleClasspath puts exported Gradle entries before config and env.
func effectiveOracleClasspath(modelEntries []string, cfg *config.Config) []string {
	return dedupePreservingOrder(append(append([]string(nil), modelEntries...), resolveOracleClasspath(cfg)...))
}

// loadGradleCompileContext supplies optional source roots and JVM target.
// The classpath loader reports malformed model warnings separately.
func loadGradleCompileContext(paths []string, explicit string, disabled bool) (dirs, generated []string, target string) {
	if disabled && explicit == "" {
		return nil, nil, ""
	}
	seenModels := map[string]bool{}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if explicit != "" {
		paths = []string{explicit}
	}
	for _, path := range paths {
		modelDir := explicit
		if modelDir == "" {
			modelDir = gradlemodel.Discover(path)
		}
		if modelDir == "" || seenModels[modelDir] {
			continue
		}
		seenModels[modelDir] = true
		model, _, err := gradlemodel.Load(modelDir)
		if err != nil {
			continue
		}
		dirs = append(dirs, model.SourceDirs()...)
		for _, project := range model.Projects {
			for _, set := range project.SourceSets {
				generated = append(generated, set.GeneratedSourceDirs...)
			}
		}
		if candidate := model.MaxJvmTarget(); candidate != "" {
			target = maxJvmTarget(target, candidate)
		}
	}
	return dedupePreservingOrder(dirs), dedupePreservingOrder(generated), target
}

func maxJvmTarget(a, b string) string {
	parse := func(s string) int {
		if len(s) > 2 && s[:2] == "1." {
			s = s[2:]
		}
		n, _ := strconv.Atoi(s)
		return n
	}
	if parse(b) > parse(a) {
		return b
	}
	return a
}
