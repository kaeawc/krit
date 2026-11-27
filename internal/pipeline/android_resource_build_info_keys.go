package pipeline

import (
	"encoding/hex"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// resourceBuildInfoRules are the active resource rules that read build files for a res directory
// (rules.ResourceBuildInfoReader).
type resourceBuildInfoRules struct {
	// ids names the rules, so the run-scoped memo tells one rule set's reads from another's.
	ids     string
	readers []rules.ResourceBuildInfoReader
}

// resourceBuildInfoRulesOf returns the active resource rules that read build files, or nil when none does. None does
// by default: every such rule is opt-in.
func resourceBuildInfoRulesOf(active []*api.Rule) *resourceBuildInfoRules {
	ids, readers := rules.ResourceBuildInfoReadersOf(active)
	if len(readers) == 0 {
		return nil
	}
	return &resourceBuildInfoRules{ids: strings.Join(ids, "\x00"), readers: readers}
}

// reads returns, sorted and unique, the lookups and files the rules read for the res directories.
func (r *resourceBuildInfoRules) reads(resDirs []string) (lookups, files []string) {
	lookupSet, fileSet := map[string]bool{}, map[string]bool{}
	for _, resDir := range resDirs {
		for _, reader := range r.readers {
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
// passes the same reads, so the manifest and resource keys share one computation per run.
func gradleInfoFingerprint(project *android.Project, reads buildFileReaders) string {
	if reads.resources == nil || project == nil || len(project.ResDirs) == 0 {
		return manifestGradleInfoFingerprint(project)
	}
	identity := reads.resources.ids + "\x01" + strings.Join(project.ManifestPaths, "\x00") + "\x01" + strings.Join(project.ResDirs, "\x00")
	return gradleInfoMemo.get(identity, func() string {
		lookups, files := reads.resources.reads(project.ResDirs)
		owners := append(append([]string(nil), project.ManifestPaths...), lookups...)
		sort.Strings(owners)
		owners = compactSorted(owners)
		h := hashutil.Hasher().New()
		_, _ = h.Write([]byte("lookups\x00" + computeManifestGradleInfoFingerprint(owners) + "\x00files\x00"))
		writeUnscannedFileHashes(h, hashutil.Default(), files, nil)
		return hex.EncodeToString(h.Sum(nil))
	})
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

// resourceBuildInfoKey is the part of a resource-rule cache key that covers the build files the active resource
// rules read (#1883): "" when none reads any, so runs without such a rule keep their keys.
func (in AndroidInput) resourceBuildInfoKey() string {
	reads := buildFileReaders{resources: resourceBuildInfoRulesOf(in.ActiveRules)}
	if reads.resources == nil {
		return ""
	}
	return "\x00build-info\x00" + gradleInfoFingerprint(in.Project, reads)
}
