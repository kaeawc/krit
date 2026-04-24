package firchecks

// invoke.go — one-shot FIR check: start daemon, send check, close.
// Used as fallback when the persistent daemon is unavailable.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
				return nil
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

// FindFirJar locates krit-fir.jar by checking standard locations relative
// to the krit binary and the project being scanned.
func FindFirJar(scanPaths []string) string {
	var candidates []string

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "krit-fir.jar"),
			filepath.Join(exeDir, "tools", "krit-fir", "build", "libs", "krit-fir.jar"),
			filepath.Join(exeDir, "..", "tools", "krit-fir", "build", "libs", "krit-fir.jar"),
		)
	}

	if len(scanPaths) > 0 {
		projectDir := scanPaths[0]
		fi, err := os.Stat(projectDir)
		if err == nil && !fi.IsDir() {
			projectDir = filepath.Dir(projectDir)
		}
		candidates = append(candidates,
			filepath.Join(projectDir, ".krit", "krit-fir.jar"),
			filepath.Join(projectDir, "tools", "krit-fir", "build", "libs", "krit-fir.jar"),
		)
	}

	cwd, _ := os.Getwd()
	candidates = append(candidates,
		filepath.Join(cwd, "tools", "krit-fir", "build", "libs", "krit-fir.jar"),
		filepath.Join(cwd, "krit-fir.jar"),
	)

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// InvokeOneShot starts a fresh krit-fir daemon, sends a single check request
// for the given files, and returns the response. The daemon is shut down
// after the request. Used when the persistent daemon is unavailable.
func InvokeOneShot(jarPath string, files []string, sourceDirs, classpath, rules []string, verbose bool) (*CheckResponse, error) {
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

	resp, err := d.Check(refs, sourceDirs, classpath, rules)
	if err != nil {
		return nil, fmt.Errorf("fir one-shot check: %w", err)
	}
	return resp, nil
}
