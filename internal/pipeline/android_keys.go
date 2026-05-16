package pipeline

import (
	"encoding/hex"
	"sort"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// resourceKey is the cache key for the dispatcher's per-resDir resource
// rules. The active resource-deps mask is folded in so toggling the
// rule set's resource needs flips the key correctly.
//
// JavaSemanticFactsFP is intentionally NOT folded in: resource rules
// dispatch on files with scanner.LangXML, and the only consumer of
// ctx.JavaSemanticFacts (semantic_rule_helpers.javaSemanticCallFact)
// hard-gates on scanner.LangJava and returns no facts otherwise. So
// rotating Java semantic facts (which happens on every .java edit
// when JavaFacts-needing rules are active) cannot change resource
// findings — including the fingerprint would force a full
// resourceRuleChecks rerun on every Java edit even though the
// resource-rule output is byte-identical. Same reasoning applies to
// the other XML-only keys below.
func (in AndroidInput) resourceKey(resDir, resDirFP string, deps rules.AndroidDataDependency, valueKinds android.ValuesScanKind) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindResources,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        resDirFP,
		// Pack (resDir path, deps mask, valueKinds mask) into Extra so
		// rule-set masks flipping invalidates the cache and so two
		// identical-content resDirs at different paths don't share an
		// entry.
		Extra: resDir + "\x00" + uintptrToHex(uint64(deps)) + "\x00" + uintptrToHex(uint64(valueKinds)),
	})
}

// uintptrToHex returns a fixed-width hex representation of v for use in
// cache key composition. Stable across architectures since we always
// emit 16 hex chars regardless of value width.
func uintptrToHex(v uint64) string {
	const digits = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = digits[v&0xf]
		v >>= 4
	}
	return string(buf[:])
}

// manifestKey / manifestBundleKey: manifest rules dispatch on
// scanner.LangXML AndroidManifest.xml files. JavaSemanticFactsFP is
// dropped for the same reason as resourceKey — manifest rules cannot
// observe Java semantic facts. See the resourceKey comment.
func (in AndroidInput) manifestKey(contentHash, bundleFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindManifest,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        contentHash,
		Extra:          bundleFP,
	})
}

func (in AndroidInput) manifestBundleKey(bundleFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindManifestBundle,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        bundleFP,
	})
}

// resourceSourceKey is the cache key for a single source file's
// resource-source rule results. srcContentHash is the file's content hash;
// mergedIdxFP covers the full merged resource index so any resource change
// invalidates every per-source entry. srcPath is folded into Extra alongside
// mergedIdxFP so that two files with identical content at different paths
// never share an entry (findings carry the file path).
func (in AndroidInput) resourceSourceKey(srcPath, srcContentHash, mergedIdxFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSource,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             srcContentHash,
		Extra:               srcPath + "\x00" + mergedIdxFP,
	})
}

// resourceBundleKey: XML-only path, same reasoning as resourceKey.
func (in AndroidInput) resourceBundleKey(mergedFP string, deps rules.AndroidDataDependency, valueKinds android.ValuesScanKind) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindResourceBundle,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        mergedFP,
		Extra:          uintptrToHex(uint64(deps)) + "\x00" + uintptrToHex(uint64(valueKinds)),
	})
}

func (in AndroidInput) resourceSourceIndexBundleKey(mergedFP string, deps rules.AndroidDataDependency, valueKinds android.ValuesScanKind) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceBundle,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             mergedFP,
		Extra:               "resource-source-index\x00" + uintptrToHex(uint64(deps)) + "\x00" + uintptrToHex(uint64(valueKinds)),
	})
}

// iconKey / iconBundleKey: icon rules walk drawable PNGs/SVGs. They
// don't run rule code against Java/Kotlin source, so the semantic-
// facts contribution is irrelevant. Same reasoning as resourceKey.
func (in AndroidInput) iconKey(resDir, resDirFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindIcons,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        resDirFP,
		Extra:          resDir,
	})
}

func (in AndroidInput) iconBundleKey(mergedFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindIconBundle,
		RuleHash:       in.RuleHash,
		LibraryFactsFP: in.LibraryFactsFP,
		InputFP:        mergedFP,
	})
}

func (in AndroidInput) resourceSourceBundleKey(sourceSetFP, mergedIdxFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             sourceSetFP,
		Extra:               mergedIdxFP,
	})
}

func (in AndroidInput) androidProjectBundleKey(needs androidPhaseNeeds, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, resDirFPs *resDirFingerprints) (string, bool) {
	if in.Project == nil || in.CacheDir == "" || in.RuleHash == "" {
		return "", false
	}
	fp, ok := in.androidProjectBundleFingerprint(needs, resourceDeps, valueKinds, resDirFPs)
	if !ok {
		return "", false
	}
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindProject,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             fp,
		Extra:               androidPhaseNeedsKey(needs),
	}), true
}

func (in AndroidInput) androidProjectBundleFingerprint(needs androidPhaseNeeds, resourceDeps rules.AndroidDataDependency, valueKinds android.ValuesScanKind, resDirFPs *resDirFingerprints) (string, bool) {
	h := hashutil.Hasher().New()
	h.Write([]byte(androidPhaseNeedsKey(needs)))
	h.Write([]byte{0})
	h.Write([]byte(uintptrToHex(uint64(resourceDeps))))
	h.Write([]byte{0})
	h.Write([]byte(uintptrToHex(uint64(valueKinds))))
	h.Write([]byte{0})
	if needs.manifest {
		if !writePathHashes(h, "manifest", in.Project.ManifestPaths) {
			return "", false
		}
	}
	if needs.gradle {
		if !writePathHashes(h, "gradle", in.Project.GradlePaths) {
			return "", false
		}
	}
	if needs.resources || needs.icons {
		writeResDirFingerprints(h, in.Project.ResDirs, resDirFPs)
	}
	if hasResourceSourceRules(in.ActiveRules) {
		sourceFP, ok := in.resourceSourceFingerprintForBundle()
		if !ok {
			return "", false
		}
		h.Write([]byte("sources"))
		h.Write([]byte{0})
		h.Write([]byte(sourceFP))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func androidPhaseNeedsKey(needs androidPhaseNeeds) string {
	var key [4]byte
	if needs.manifest {
		key[0] = 'm'
	}
	if needs.resources {
		key[1] = 'r'
	}
	if needs.icons {
		key[2] = 'i'
	}
	if needs.gradle {
		key[3] = 'g'
	}
	return string(key[:])
}

func writePathHashes(h interface {
	Write([]byte) (int, error)
}, label string, paths []string) bool {
	memo := hashutil.Default()
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	_, _ = h.Write([]byte(label))
	_, _ = h.Write([]byte{0})
	for _, path := range sorted {
		_, _ = h.Write([]byte(path))
		_, _ = h.Write([]byte{0})
		fileHash, err := memo.HashFile(path, nil)
		if err != nil {
			return false
		}
		_, _ = h.Write([]byte(fileHash))
		_, _ = h.Write([]byte{0})
	}
	return true
}

func writeResDirFingerprints(h interface {
	Write([]byte) (int, error)
}, resDirs []string, fps *resDirFingerprints) {
	sorted := append([]string(nil), resDirs...)
	sort.Strings(sorted)
	_, _ = h.Write([]byte("res"))
	_, _ = h.Write([]byte{0})
	for _, resDir := range sorted {
		_, _ = h.Write([]byte(resDir))
		_, _ = h.Write([]byte{0})
		var fp string
		if fps != nil {
			fp = fps.fingerprint(resDir)
		} else {
			fp = resDirContentFingerprint(resDir)
		}
		_, _ = h.Write([]byte(fp))
		_, _ = h.Write([]byte{0})
	}
}

func (in AndroidInput) resourceSourceFingerprintForBundle() (string, bool) {
	if len(in.SourceFiles) > 0 {
		return resourceSourceSetFingerprint(in.SourceFiles)
	}
	if len(in.SourcePaths) > 0 && len(in.SourceHashes) > 0 {
		entries := make([]resourceSourceEntry, 0, len(in.SourcePaths))
		for _, path := range in.SourcePaths {
			hash := in.SourceHashes[path]
			if path == "" || hash == "" {
				return "", false
			}
			entries = append(entries, resourceSourceEntry{path: path, hash: hash})
		}
		return resourceSourceEntriesFingerprint(entries)
	}
	if len(in.SourcePaths) > 0 {
		return resourceSourceSetFingerprintFromPaths(in.SourcePaths)
	}
	return "", false
}

func (in AndroidInput) resourceSourceBundleManifestKey(paths []string, mergedIdxFP string) (string, bool) {
	pathSetFP, ok := resourceSourcePathSetFingerprint(paths)
	if !ok {
		return "", false
	}
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             pathSetFP,
		Extra:               "manifest\x00" + mergedIdxFP,
	}), true
}

func (in AndroidInput) gradleKey(path, contentHash string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindGradle,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		// File path is folded in alongside the content hash because two
		// Gradle scripts can have identical bodies in different
		// positions in the project tree (root vs module) and rules may
		// behave differently based on which one they're inspecting.
		InputFP: contentHash,
		Extra:   path,
	})
}

func (in AndroidInput) gradleBundleKey(bundleFP string) string {
	return scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindGradleBundle,
		RuleHash:            in.RuleHash,
		LibraryFactsFP:      in.LibraryFactsFP,
		JavaSemanticFactsFP: in.JavaSemanticFactsFP,
		InputFP:             bundleFP,
	})
}
