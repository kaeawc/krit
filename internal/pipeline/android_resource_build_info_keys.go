package pipeline

import (
	"encoding/hex"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/sysres"
)

// resourceBuildInfoRules are the active resource rules that read build files for a res directory
// (rules.ResourceBuildInfoReader).
type resourceBuildInfoRules struct {
	// ids names the rules, so the run-scoped memo tells one rule set's reads from another's.
	ids     string
	readers []rules.ResourceBuildInfoReader
	// local are the readers whose findings for a res directory depend only on what they read for it
	// (rules.ResourceBuildInfoLocalReader); project are the others, whose reads for every res directory key each.
	local, project []rules.ResourceBuildInfoReader
}

// resourceBuildInfoRulesOf returns the active resource rules that read build files, or nil when none does.
func resourceBuildInfoRulesOf(active []*api.Rule) *resourceBuildInfoRules {
	ids, readers := rules.ResourceBuildInfoReadersOf(active)
	if len(readers) == 0 {
		return nil
	}
	r := &resourceBuildInfoRules{ids: strings.Join(ids, "\x00"), readers: readers}
	for _, reader := range readers {
		if _, ok := reader.(rules.ResourceBuildInfoLocalReader); ok {
			r.local = append(r.local, reader)
		} else {
			r.project = append(r.project, reader)
		}
	}
	return r
}

// reads returns, sorted and unique, the lookups and files the rules read for the res directories.
func (r *resourceBuildInfoRules) reads(resDirs []string) (lookups, files []string) {
	return buildInfoReadsOf(r.readers, resDirs)
}

// buildInfoReadsOf returns, sorted and unique, the lookups and files readers read for the res directories.
func buildInfoReadsOf(readers []rules.ResourceBuildInfoReader, resDirs []string) (lookups, files []string) {
	lookupSet, fileSet := map[string]bool{}, map[string]bool{}
	for _, resDir := range resDirs {
		for _, reader := range readers {
			reads := reader.ResourceBuildInfoReads(resDir)
			for _, lookup := range reads.Lookups {
				lookupSet[lookup] = true
			}
			for _, file := range reads.Files {
				fileSet[file] = true
			}
		}
	}
	return sortedKeys(lookupSet), sortedKeys(fileSet)
}

// gradleInfoMemo shares gradleInfoFingerprint between the keys of one run that has a resource rule reading build
// files.
var gradleInfoMemo runScoped[string]

// gradleInfoFingerprint hashes the build files the active manifest and resource rules read beyond the files of the
// scan. Without an active resource rule that reads build files it is manifestGradleInfoFingerprint. With one (#1883),
// that rule's lookups for every res directory are hashed together with the manifests, through the same
// computeManifestGradleInfoFingerprint: the build script each reads, its content, and the version catalogs its
// aliases resolve through; then the content, or absence, of every file such a rule reads itself. Every key of a run
// passes the same reads, so the manifest and whole-project resource keys share one computation per run.
func gradleInfoFingerprint(project *android.Project, reads buildFileReaders) string {
	if reads.resources == nil || project == nil || len(project.ResDirs) == 0 {
		return manifestGradleInfoFingerprint(project)
	}
	identity := reads.resources.ids + "\x01" + strings.Join(project.ManifestPaths, "\x00") + "\x01" + strings.Join(project.ResDirs, "\x00")
	return gradleInfoMemo.get(identity, func() string {
		lookups, files := reads.resources.reads(project.ResDirs)
		owners := append(append([]string(nil), project.ManifestPaths...), lookups...)
		sort.Strings(owners)
		return buildInfoReadsFingerprint(newGradleInfoResolver(), sysres.CPUs(), compactSorted(owners), files)
	})
}

// buildInfoReadsFingerprint hashes what is read through the lookups (sorted and unique), as
// computeManifestGradleInfoFingerprint does for manifests, resolving them through resolver across workers, and the
// content, or absence, of the files read directly.
func buildInfoReadsFingerprint(resolver *gradleInfoResolver, workers int, lookups, files []string) string {
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("lookups\x00" + resolver.fingerprint(lookups, workers) + "\x00files\x00"))
	writeUnscannedFileHashes(h, hashutil.Default(), files, nil)
	return hex.EncodeToString(h.Sum(nil))
}

func compactSorted(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// resourceBuildInfoKeys computes each build-info part of one run's resource cache keys once, however many res
// directories and keys ask for it: the whole-project part the resource bundle and Android project bundle keys take,
// and each res directory's part. Run installs one (withResourceBuildInfoKeys) that every copy of the input shares.
// Before, every key of every res directory rebuilt the memo identity of gradleInfoFingerprint, every manifest and res
// path joined, so a run built O(res dirs²) bytes of it (#1943).
type resourceBuildInfoKeys struct {
	project *android.Project
	active  []*api.Rule

	rulesOnce sync.Once
	rules     *resourceBuildInfoRules

	wholeOnce sync.Once
	whole     string

	projectOnce sync.Once
	projectFP   string

	// resolver resolves each lookup's build script and each script's catalogs once for every res directory's key.
	resolver *gradleInfoResolver
	mu       sync.Mutex
	dirs     map[string]string
}

func newResourceBuildInfoKeys(project *android.Project, active []*api.Rule) *resourceBuildInfoKeys {
	return &resourceBuildInfoKeys{project: project, active: active, resolver: newGradleInfoResolver(), dirs: map[string]string{}}
}

// withResourceBuildInfoKeys installs the run's resourceBuildInfoKeys. Every copy of in shares it, so the parallel
// res-dir workers compute each part once.
func (in AndroidInput) withResourceBuildInfoKeys() AndroidInput {
	in.buildInfoKeys = newResourceBuildInfoKeys(in.Project, in.ActiveRules)
	return in
}

// resourceBuildInfo returns the run's resourceBuildInfoKeys, or a fresh one for an input Run has not prepared.
func (in AndroidInput) resourceBuildInfo() *resourceBuildInfoKeys {
	if in.buildInfoKeys != nil {
		return in.buildInfoKeys
	}
	return newResourceBuildInfoKeys(in.Project, in.ActiveRules)
}

func (k *resourceBuildInfoKeys) readers() *resourceBuildInfoRules {
	k.rulesOnce.Do(func() { k.rules = resourceBuildInfoRulesOf(k.active) })
	return k.rules
}

// wholeKey is the part of a whole-project resource key (the resource bundle, the Android project bundle) that covers
// the build files the active resource rules read for every res directory (#1883): "" when none reads any, so runs
// without such a rule keep their keys.
func (k *resourceBuildInfoKeys) wholeKey() string {
	k.wholeOnce.Do(func() {
		if r := k.readers(); r != nil {
			k.whole = "\x00build-info\x00" + gradleInfoFingerprint(k.project, buildFileReaders{resources: r})
		}
	})
	return k.whole
}

// dirKey is the part of a res directory's resource key that covers the build files the active resource rules read
// for its findings: what each local reader reads for that directory, and what each other reader reads for every res
// directory of the project. With only local readers active, an edit of one module's build script moves the keys of
// that module's res directories alone, while a file several modules read, such as a version catalog their aliases
// resolve through or the settings script, moves the key of each that reads it (#1943). "" when no active resource
// rule reads build files.
func (k *resourceBuildInfoKeys) dirKey(resDir string) string {
	r := k.readers()
	if r == nil {
		return ""
	}
	if k.project == nil || len(k.project.ResDirs) == 0 {
		return k.wholeKey()
	}
	k.mu.Lock()
	key, ok := k.dirs[resDir]
	k.mu.Unlock()
	if ok {
		return key
	}
	k.projectOnce.Do(func() {
		if len(r.project) > 0 {
			lookups, files := buildInfoReadsOf(r.project, k.project.ResDirs)
			k.projectFP = buildInfoReadsFingerprint(k.resolver, sysres.CPUs(), lookups, files)
		}
	})
	var local string
	if len(r.local) > 0 {
		// A res directory's reads are a few paths, resolved on the calling res-dir worker.
		lookups, files := buildInfoReadsOf(r.local, []string{resDir})
		local = buildInfoReadsFingerprint(k.resolver, 1, lookups, files)
	}
	key = "\x00build-info-dir\x00" + k.projectFP + "\x00" + local
	k.mu.Lock()
	k.dirs[resDir] = key
	k.mu.Unlock()
	return key
}

// resourceBuildInfoKey is the whole-project build-info part of a resource cache key (resourceBuildInfoKeys.wholeKey).
func (in AndroidInput) resourceBuildInfoKey() string {
	return in.resourceBuildInfo().wholeKey()
}

// resourceDirBuildInfoKey is the build-info part of resDir's resource cache key (resourceBuildInfoKeys.dirKey).
func (in AndroidInput) resourceDirBuildInfoKey(resDir string) string {
	return in.resourceBuildInfo().dirKey(resDir)
}
