// Package clishared holds helpers shared across krit's verb-specific
// CLI packages (internal/cli/<verb>/).
package clishared

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

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
		f, err := scanner.ParseFile(path)
		if err != nil {
			continue
		}
		files = append(files, f)
	}
	return files
}
