package scan

import (
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/config"
)

// resolveOracleClasspath assembles the user-configured classpath
// the FIR daemon's `analyze` RPC receives. Order of precedence:
//
//  1. `oracle.classpath` from krit.yml
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
