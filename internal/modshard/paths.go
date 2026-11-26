package modshard

// paths.go — spell the Gradle model's directories the way the scan does (#1674).
//
// Gradle records resolved paths (on macOS /tmp is /private/tmp), while a scan
// path can reach the same directory through a symlink. Planning compares the
// model's source roots with the scan paths, and their files with the check
// request's, as strings, so with two spellings nothing matched and the
// settings split, the source-set isolation and the module oracle silently did
// not apply.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

type scanSpelling struct{ abs, resolved string }

// alignModelPaths rewrites the project and source directories of model that
// are another spelling (through a symlink) of a directory at, under or above a
// scan path into the scan path's spelling. Classpath entries keep the model's
// spelling: the check request lists them that way.
func alignModelPaths(model *gradlemodel.Model, scanPaths []string) {
	if model == nil {
		return
	}
	var scans []scanSpelling
	for _, sp := range scanPaths {
		abs, err := filepath.Abs(sp)
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			scans = append(scans, scanSpelling{abs, filepath.Clean(resolved)})
		}
	}
	// respelled maps a directory as the model spells it to the scan's spelling.
	respelled := map[string]string{}
	for _, s := range scans {
		if s.resolved != s.abs {
			respelled[s.resolved] = s.abs
		}
	}
	resolve := newDirResolver()
	for _, p := range model.Projects {
		if p.Dir == "" {
			continue
		}
		dir := filepath.Clean(p.Dir)
		if lexicallyRelated(dir, scans) || respellPath(dir, respelled) != dir {
			continue // spelled like a scan path, or a resolved scan path maps it
		}
		resolved, ok := resolve(dir)
		if !ok {
			continue
		}
		for _, s := range scans {
			if spelled, ok := respell(resolved, s); ok {
				if spelled != dir {
					respelled[dir] = spelled
				}
				break
			}
		}
	}
	if len(respelled) == 0 {
		return
	}
	for i := range model.Projects {
		p := &model.Projects[i]
		p.Dir = respellPath(p.Dir, respelled)
		for j := range p.SourceSets {
			s := &p.SourceSets[j]
			for k := range s.SourceDirs {
				s.SourceDirs[k] = respellPath(s.SourceDirs[k], respelled)
			}
			for k := range s.GeneratedSourceDirs {
				s.GeneratedSourceDirs[k] = respellPath(s.GeneratedSourceDirs[k], respelled)
			}
		}
	}
}

// newDirResolver returns filepath.EvalSymlinks for directories, memoized per
// directory so sibling projects resolve their shared parents once and cost
// one Lstat each.
func newDirResolver() func(dir string) (string, bool) {
	memo := map[string]string{}
	var resolve func(dir string) (string, bool)
	resolve = func(dir string) (string, bool) {
		if r, ok := memo[dir]; ok {
			return r, r != ""
		}
		parent := filepath.Dir(dir)
		var r string
		if parent == dir {
			r = dir
		} else if rp, ok := resolve(parent); ok {
			if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink == 0 {
				r = filepath.Join(rp, filepath.Base(dir))
			} else if err == nil {
				if target, err := filepath.EvalSymlinks(dir); err == nil {
					r = filepath.Clean(target)
				}
			}
		}
		memo[dir] = r
		return r, r != ""
	}
	return resolve
}

// lexicallyRelated reports whether dir is at, under or above a scan path as
// spelled, so it needs no symlink resolution.
func lexicallyRelated(dir string, scans []scanSpelling) bool {
	for _, s := range scans {
		if within(dir, s.abs) || within(s.abs, dir) {
			return true
		}
	}
	return false
}

// respell returns resolved (a resolved directory) in the spelling of scan when
// it is at, under or above the scan path.
func respell(resolved string, scan scanSpelling) (string, bool) {
	if within(resolved, scan.resolved) {
		return scan.abs + resolved[len(scan.resolved):], true
	}
	if within(scan.resolved, resolved) {
		// The scan path is below resolved; its spelling keeps the same tail
		// unless the symlink sits inside that tail.
		tail := scan.resolved[len(resolved):]
		if strings.HasSuffix(scan.abs, tail) {
			return scan.abs[:len(scan.abs)-len(tail)], true
		}
	}
	return "", false
}

// within reports whether path is dir or under it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}

// respellPath replaces the deepest directory of path that respelled maps.
func respellPath(path string, respelled map[string]string) string {
	clean := filepath.Clean(path)
	// Walk the parents as prefixes of clean, without allocating.
	for end := len(clean); end > 0; end = strings.LastIndexByte(clean[:end], filepath.Separator) {
		if spelled, ok := respelled[clean[:end]]; ok {
			return spelled + clean[end:]
		}
	}
	return path
}
