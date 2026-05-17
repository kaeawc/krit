// Package clishared holds helpers shared across krit's verb-specific
// CLI packages (internal/cli/<verb>/).
package clishared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/snapshot"
)

// ResolveRepoRoot honors --repo, falling back to cwd. Returns the
// resolved root and an exit code; non-zero exit means an error has
// already been reported to stderr.
func ResolveRepoRoot(flagValue string) (string, int) {
	if flagValue != "" {
		return flagValue, 0
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return "", 1
	}
	return cwd, 0
}

// ResolveCommitOrLiteral tries `git rev-parse ref` inside repoRoot and
// falls back to the literal ref on failure. Captured snapshots may
// outlive a branch, so a literal sha needs to keep working without
// git.
func ResolveCommitOrLiteral(repoRoot, ref string) string {
	if ref == "" {
		return ""
	}
	if sha, err := snapshot.ResolveCommitSHA(repoRoot, ref); err == nil {
		return sha
	}
	return ref
}

// ShortSHA returns the first 12 chars of a commit sha, or the input
// unchanged when shorter. Matches `git rev-parse --short=12`.
func ShortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// FindConfigInDir probes dir for the first present file in
// config.Filenames and returns its path, or "" when none is present
// (or dir is empty).
func FindConfigInDir(dir string) string {
	if dir == "" {
		return ""
	}
	for _, name := range config.Filenames {
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}
	return ""
}

// ParseRuleNameSetCSV parses a comma-separated list of rule names into a
// lookup set. Whitespace around each name is trimmed. Empty input returns
// an empty (non-nil) map so callers can index it without nil checks.
//
// Shared by --disable-rules / --enable-rules on both the CLI and daemon
// paths. A trailing comma or whitespace-only token produces an empty-string
// entry — harmless because no real rule has ID "".
func ParseRuleNameSetCSV(csv string) map[string]bool {
	out := make(map[string]bool)
	if csv == "" {
		return out
	}
	for _, name := range strings.Split(csv, ",") {
		out[strings.TrimSpace(name)] = true
	}
	return out
}

// SimpleName extracts the last dot-separated segment of an FQN,
// stripping any trailing parenthesised arity/signature.
func SimpleName(fqn string) string {
	if i := strings.IndexByte(fqn, '('); i >= 0 {
		fqn = fqn[:i]
	}
	if i := strings.LastIndexByte(fqn, '.'); i >= 0 {
		return fqn[i+1:]
	}
	return fqn
}

// SplitPositional pulls up to max leading non-flag arguments out of
// args and returns them alongside the remaining (flag) arguments. Once
// a flag appears, all subsequent arguments are treated as flag args.
func SplitPositional(args []string, maxVal int) (positional, rest []string) {
	rest = make([]string, 0, len(args))
	for _, arg := range args {
		if len(positional) < maxVal && !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		rest = append(rest, arg)
	}
	return positional, rest
}

// MultiString is a flag.Value that accumulates repeated --flag value
// invocations into a slice.
type MultiString []string

func (m *MultiString) String() string {
	return strings.Join(*m, ",")
}

func (m *MultiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// ScanModuleKotlinFiles finds and parses all .kt files under the
// module's source roots.
func ScanModuleKotlinFiles(mod *module.Module) []*scanner.File {
	var ktFiles []string
	roots := mod.SourceRoots
	if len(roots) == 0 {
		roots = []string{filepath.Join(mod.Dir, "src", "main", "kotlin")}
	}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if !info.IsDir() && strings.HasSuffix(path, ".kt") {
				ktFiles = append(ktFiles, path)
			}
			return nil
		})
	}
	files := make([]*scanner.File, 0, len(ktFiles))
	for _, path := range ktFiles {
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			continue
		}
		files = append(files, f)
	}
	return files
}
