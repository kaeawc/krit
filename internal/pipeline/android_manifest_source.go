package pipeline

import (
	"context"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// parsedManifests memoizes manifest parsing within one Android phase run so
// the manifest loop and the manifest-source pass parse each manifest once.
type parsedManifests struct {
	mu    sync.Mutex
	byKey map[string]parsedManifestEntry
}

type parsedManifestEntry struct {
	m   *rules.Manifest
	err error
}

func (p *parsedManifests) get(path string) (*rules.Manifest, error) {
	if p == nil {
		return parseManifestForRules(path)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.byKey[path]; ok {
		return e.m, e.err
	}
	m, err := parseManifestForRules(path)
	if p.byKey == nil {
		p.byKey = map[string]parsedManifestEntry{}
	}
	p.byKey[path] = parsedManifestEntry{m: m, err: err}
	return m, err
}

func parseManifestForRules(path string) (*rules.Manifest, error) {
	parsed, err := android.ParseManifest(path)
	if err != nil {
		return nil, err
	}
	return ConvertManifestForRules(android.ConvertManifest(parsed, path)), nil
}

// hasManifestSourceRules reports whether any active rule joins parsed source
// with the project's manifests (api.ScopeManifestSource).
func hasManifestSourceRules(activeRules []*api.Rule) bool {
	for _, r := range activeRules {
		if r != nil && rules.RuleScope(r) == api.ScopeManifestSource {
			return true
		}
	}
	return false
}

// manifestSourceSourceFingerprint hashes the (path, content hash) pair of
// every project source file: the parsed files plus any path the parse phase
// skipped as a findings-cache hit.
func (in AndroidInput) manifestSourceSourceFingerprint() (string, bool) {
	memo := hashutil.Default()
	hashes := map[string]string{}
	for _, f := range in.SourceFiles {
		if f == nil || f.Path == "" {
			continue
		}
		hash := in.SourceHashes[f.Path]
		if hash == "" {
			var provider func() ([]byte, error)
			if len(f.Content) > 0 {
				content := f.Content
				provider = func() ([]byte, error) { return content, nil }
			}
			h, err := memo.HashFile(f.Path, provider)
			if err != nil {
				return "", false
			}
			hash = h
		}
		hashes[f.Path] = hash
	}
	for _, path := range in.SourcePaths {
		if path == "" || hashes[path] != "" {
			continue
		}
		hash := in.SourceHashes[path]
		if hash == "" {
			h, err := memo.HashFile(path, nil)
			if err != nil {
				return "", false
			}
			hash = h
		}
		hashes[path] = hash
	}
	paths := make([]string, 0, len(hashes))
	for path := range hashes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := hashutil.Hasher().New()
	for _, path := range paths {
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write([]byte(hashes[path]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// manifestSourceGradleFingerprint hashes the settings files and build scripts manifest-source rules read from disk:
// the module's plugins and namespace, and ProviderReadPermissionOnly's module graph, which decides which apps merge
// a library manifest and can reach build files outside the scan paths (#1625, #1800). Groovy build.gradle files are
// not source files, so the source fingerprint misses them. See buildFilesFingerprint.
func (in AndroidInput) manifestSourceGradleFingerprint() (string, bool) {
	return buildFilesFingerprint(in.Project, rules.ReadsModuleGraph(in.ActiveRules))
}

func (in AndroidInput) manifestSourceKey(manifestFP, sourceFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindManifestSource,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             manifestFP,
		Extra:               sourceFP,
	})
}

// manifestSourceFiles returns the complete project source set. When the parse
// phase skipped findings-cache hits, the skipped files are parsed here: the
// rules are promised every Kotlin and Java file, and this only runs on a
// manifest-source cache miss.
func (in AndroidInput) manifestSourceFiles() []*scanner.File {
	files := make([]*scanner.File, 0, len(in.SourceFiles))
	have := make(map[string]bool, len(in.SourceFiles))
	for _, f := range in.SourceFiles {
		if f != nil && !have[f.Path] {
			have[f.Path] = true
			files = append(files, f)
		}
	}
	for _, path := range in.SourcePaths {
		if path == "" || have[path] {
			continue
		}
		have[path] = true
		var f *scanner.File
		var err error
		switch {
		case strings.HasSuffix(path, ".java"):
			f, err = scanner.ParseJavaFile(context.Background(), path)
		case strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts"):
			f, err = scanner.ParseFile(context.Background(), path)
		default:
			continue
		}
		if err == nil && f != nil {
			files = append(files, f)
		}
	}
	return files
}

// runManifestSourcePass runs the ScopeManifestSource rules once for the
// project. The result is cached under every manifest's content hash, every
// settings file and build script, and the full source-set fingerprint, so
// editing any of them re-runs the rules.
func (p AndroidPhase) runManifestSourcePass(in AndroidInput, manifests *parsedManifests, collector *scanner.FindingCollector, tracker perf.Tracker) {
	if in.Dispatcher == nil || len(in.Dispatcher.ManifestSourceRules()) == 0 || len(in.Project.ManifestPaths) == 0 {
		return
	}
	start := time.Now()
	defer func() { perf.AddEntry(tracker, "manifestSourceRuleChecks", time.Since(start)) }()

	key := ""
	if in.CacheDir != "" && in.CacheWriter != nil && in.RuleHash != "" {
		sourceFP, ok := in.manifestSourceSourceFingerprint()
		var gradleFP string
		if ok {
			gradleFP, ok = in.manifestSourceGradleFingerprint()
		}
		if ok {
			key = in.manifestSourceKey(p.manifestBundleFingerprint(in.Project.ManifestPaths)+"\x00"+gradleFP, sourceFP)
			if cached, hit := scanner.LoadAndroidFindings(in.CacheDir, key); hit {
				collector.AppendColumns(&cached)
				return
			}
		}
	}

	parsed := make([]*rules.Manifest, 0, len(in.Project.ManifestPaths))
	for _, path := range in.Project.ManifestPaths {
		if m, err := manifests.get(path); err == nil && m != nil {
			parsed = append(parsed, m)
		}
	}
	cols, cacheable := in.Dispatcher.RunManifestSource(parsed, in.manifestSourceFiles())
	collector.AppendColumns(&cols)
	if key != "" && cacheable {
		in.CacheWriter.Save(in.CacheDir, key, cols)
	}
}
