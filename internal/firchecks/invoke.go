package firchecks

// invoke.go — one-shot FIR check: start daemon, send check, close.
// Used as fallback when the persistent daemon is unavailable.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
)

// CollectFirKtFiles walks the given scan paths and returns all .kt files,
// skipping standard build/hidden directories. Used to build the file list
// for InvokeCached when no explicit list is provided.
func CollectFirKtFiles(scanPaths []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, root := range scanPaths {
		fi, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !fi.IsDir() {
			if strings.HasSuffix(root, ".kt") || strings.HasSuffix(root, ".kts") {
				if !seen[root] {
					seen[root] = true
					out = append(out, root)
				}
			}
			continue
		}
		err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if info.IsDir() {
				base := filepath.Base(p)
				if base == ".gradle" || base == ".git" || base == "build" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := info.Name()
			if !strings.HasSuffix(name, ".kt") && !strings.HasSuffix(name, ".kts") {
				return nil
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FindFirJar locates krit-fir.jar without downloading; see
// oracle.FindBackendJar for the lookup order (KRIT_FIR_JAR, ~/.krit/jars,
// next to the binary, the project, and the in-tree build output).
func FindFirJar(scanPaths []string) string {
	return oracle.FindBackendJar(oracle.BackendFIR, scanPaths)
}

// InvokeOneShot starts a fresh krit-fir daemon, sends a single check request
// for the given files, and returns the response. The daemon is shut down
// after the request. Used when the persistent daemon is unavailable.
func InvokeOneShot(jarPath string, files []string, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts, verbose bool) (*CheckResponse, error) {
	d, err := StartFirDaemonWithPort(jarPath, verbose)
	if err != nil {
		return nil, fmt.Errorf("fir one-shot start: %w", err)
	}
	defer d.Close()

	refs := make([]fileRef, 0, len(files))
	for _, p := range files {
		hash, herr := ContentHash(p)
		if herr != nil {
			hash = ""
		}
		refs = append(refs, fileRef{Path: p, ContentHash: hash})
	}

	resp, err := d.Check(refs, sourceDirs, classpath, rules, ruleConfigs, facts)
	if err != nil {
		return nil, fmt.Errorf("fir one-shot check: %w", err)
	}
	return resp, nil
}
