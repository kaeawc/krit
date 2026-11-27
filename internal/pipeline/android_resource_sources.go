package pipeline

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/scanner"
)

// resourceRuleSources gives a resource rule that reads the parsed sources
// (OnClick) the whole source set, and fingerprints that set for the caches of
// its output (#1860). A warm run that parsed only its cache misses has the
// rest parsed here, on the first res directory such a rule reads sources for;
// a directory none reads them for never asks.
type resourceRuleSources struct {
	parsed []*scanner.File
	paths  []string
	hashes map[string]string
	parse  func(paths []string) ([]*scanner.File, error)

	filesOnce sync.Once
	files     []*scanner.File
	filesOK   bool
	asked     bool

	fpOnce sync.Once
	fp     string
	fpOK   bool
	// used is the content hash fingerprint took for each path.
	used map[string]string
}

// withResourceRuleSources installs the run's resourceRuleSources. Every copy
// of in shares it, so the parallel res-dir workers parse and hash once.
func (in AndroidInput) withResourceRuleSources() AndroidInput {
	in.ruleSources = &resourceRuleSources{
		parsed: in.SourceFiles,
		paths:  in.SourcePaths,
		hashes: in.SourceHashes,
		parse:  in.ParseSources,
	}
	return in
}

// resourceRuleSources returns the run's shared view, or a fresh one for an
// input Run has not prepared.
func (in AndroidInput) resourceRuleSources() *resourceRuleSources {
	if in.ruleSources != nil {
		return in.ruleSources
	}
	return in.withResourceRuleSources().ruleSources
}

// sources returns every source file, parsing the ones the run left out. ok
// is false when a left-out file could not be parsed; the findings computed
// from the rest are then not cached.
func (s *resourceRuleSources) sources() ([]*scanner.File, bool) {
	if s == nil {
		return nil, false
	}
	s.filesOnce.Do(func() {
		s.asked = true
		s.files, s.filesOK = s.parsed, true
		// A skipped parse leaves the cache hits as stubs without a tree.
		var parsed []*scanner.File
		have := make(map[string]bool, len(s.parsed))
		for _, f := range s.parsed {
			if f != nil && f.FlatTree != nil {
				parsed = append(parsed, f)
				have[f.Path] = true
			}
		}
		var missing []string
		for _, path := range s.paths {
			if !have[path] {
				missing = append(missing, path)
			}
		}
		if len(missing) == 0 {
			return
		}
		if s.parse == nil {
			s.filesOK = false
			return
		}
		// The parse drops what a full parse would (generated files), and a
		// file deleted since the walk fails fingerprint, so nothing is cached.
		more, err := s.parse(missing)
		if err != nil || !s.matchesFingerprint(more) {
			s.filesOK = false
		}
		s.files = append(parsed, more...)
	})
	return s.files, s.filesOK
}

// cacheable reports whether every rule that read the sources this run got a
// result worth caching: true when none read them. Call it once the readers
// are done (the resource subphase has returned).
func (s *resourceRuleSources) cacheable() bool {
	return s == nil || !s.asked || s.filesOK
}

// matchesFingerprint reports whether files hold the content fingerprint
// hashed for their paths. A file edited between the two reads would leave
// the findings describing bytes the key does not, so they are not cached.
func (s *resourceRuleSources) matchesFingerprint(files []*scanner.File) bool {
	if _, ok := s.fingerprint(); !ok {
		return false
	}
	for _, f := range files {
		if f == nil {
			continue
		}
		used, ok := s.used[f.Path]
		if !ok || canonResourceSourceHash(hashutil.HashHex(f.Content)) != canonResourceSourceHash(used) {
			return false
		}
	}
	return true
}

// fingerprint covers every source path and its content, the unparsed ones of
// a partial parse included, so it is the same whether or not this run parsed
// a file.
func (s *resourceRuleSources) fingerprint() (string, bool) {
	if s == nil {
		return "", false
	}
	s.fpOnce.Do(func() {
		if len(s.paths) == 0 {
			s.fp, s.fpOK = resourceSourceSetFingerprint(s.parsed)
			return
		}
		content := make(map[string][]byte, len(s.parsed))
		for _, f := range s.parsed {
			if f != nil && len(f.Content) > 0 {
				content[f.Path] = f.Content
			}
		}
		memo := hashutil.Default()
		s.used = make(map[string]string, len(s.paths))
		entries := make([]resourceSourceEntry, 0, len(s.paths))
		for _, path := range s.paths {
			hash := s.hashes[path]
			if hash == "" {
				var provider func() ([]byte, error)
				if c, ok := content[path]; ok {
					provider = func() ([]byte, error) { return c, nil }
				}
				h, err := memo.HashFile(path, provider)
				if err != nil {
					return
				}
				hash = h
			}
			s.used[path] = hash
			entries = append(entries, resourceSourceEntry{path: path, hash: hash})
		}
		s.fp, s.fpOK = resourceSourceEntriesFingerprint(entries)
	})
	return s.fp, s.fpOK
}

// splitSourcePaths separates Kotlin from Java source paths for
// parseFilesByPath; other extensions are dropped.
func splitSourcePaths(paths []string) (kotlin, java []string) {
	kotlin, java = []string{}, []string{}
	for _, path := range paths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".kt", ".kts":
			kotlin = append(kotlin, path)
		case ".java":
			java = append(java, path)
		}
	}
	return kotlin, java
}
