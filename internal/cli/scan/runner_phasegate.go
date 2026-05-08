package scan

import (
	"github.com/kaeawc/krit/internal/cache"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func shouldOpenResourceIndexCache(activeRules []*api.Rule, noResourceCache bool, skipSourceParse bool, androidFindingsCacheable bool) bool {
	if noResourceCache || !rulesNeedAndroidProject(activeRules) {
		return false
	}
	if skipSourceParse && androidFindingsCacheable {
		return false
	}
	return true
}

func canParseOnlyCacheMisses(activeRules []*api.Rule, cacheResult *cache.Result, useCache bool, allowCrossFileDelta bool, allowResourceSourceDelta bool) bool {
	if !useCache || cacheResult == nil || cacheResult.TotalCached == 0 {
		return false
	}
	return !rulesNeedParsedSource(activeRules, allowCrossFileDelta, allowResourceSourceDelta)
}

func cacheMissPaths(paths []string, cacheResult *cache.Result) []string {
	if cacheResult == nil || len(cacheResult.CachedPaths) == 0 {
		return append([]string(nil), paths...)
	}
	capHint := len(paths) - len(cacheResult.CachedPaths)
	if capHint < 0 {
		capHint = 0
	}
	misses := make([]string, 0, capHint)
	for _, path := range paths {
		if !cacheResult.CachedPaths[path] {
			misses = append(misses, path)
		}
	}
	return misses
}

func parsedSourceBlockReason(activeRules []*api.Rule, allowCrossFileWithoutParsed bool, allowResourceSourceWithoutParsed bool) string {
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		if resourceBackedSourceRule(ru) && !allowResourceSourceWithoutParsed {
			return ru.ID + " needs resource-backed source"
		}
		if !allowCrossFileWithoutParsed && ru.Needs.Has(api.NeedsParsedFiles) {
			return ru.ID + " needs parsed files"
		}
		if !allowCrossFileWithoutParsed && ru.Needs.Has(api.NeedsCrossFile) {
			return ru.ID + " needs cross-file source"
		}
		if ru.Needs.Has(api.NeedsAggregate) {
			return ru.ID + " is aggregate"
		}
		if ru.JavaFacts != nil {
			return ru.ID + " needs Java semantic facts"
		}
	}
	if api.NeedsJavaFacts(activeRules) {
		return "Java semantic facts"
	}
	return "unknown"
}

func rulesNeedParsedSource(activeRules []*api.Rule, allowCrossFileWithoutParsed bool, allowResourceSourceWithoutParsed bool) bool {
	caps := api.Capabilities(0)
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		caps |= ru.Needs
		if resourceBackedSourceRule(ru) && !allowResourceSourceWithoutParsed {
			return true
		}
	}
	if (!allowCrossFileWithoutParsed && caps.Has(api.NeedsParsedFiles)) ||
		(!allowCrossFileWithoutParsed && caps.Has(api.NeedsCrossFile)) ||
		caps.Has(api.NeedsAggregate) {
		return true
	}
	return api.NeedsJavaFacts(activeRules)
}

func hasResourceSourceRulesForSkip(activeRules []*api.Rule) bool {
	for _, ru := range activeRules {
		if resourceBackedSourceRule(ru) {
			return true
		}
	}
	return false
}

func rulesNeedCrossOrParsedFiles(activeRules []*api.Rule) bool {
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		if ru.Needs.Has(api.NeedsCrossFile) || ru.Needs.Has(api.NeedsParsedFiles) {
			return true
		}
	}
	return false
}

func resourceBackedSourceRule(ru *api.Rule) bool {
	if ru == nil || !ru.Needs.Has(api.NeedsResources) || len(ru.NodeTypes) == 0 {
		return false
	}
	for _, lang := range api.RuleLanguages(ru) {
		if lang != scanner.LangXML {
			return true
		}
	}
	return false
}

func rulesNeedAndroidProject(activeRules []*api.Rule) bool {
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		if ru.AndroidDeps != 0 ||
			ru.Needs.Has(api.NeedsManifest) ||
			ru.Needs.Has(api.NeedsResources) ||
			ru.Needs.Has(api.NeedsGradle) {
			return true
		}
	}
	return false
}

func rulesNeedProjectModel(activeRules []*api.Rule) bool {
	if rulesNeedAndroidProject(activeRules) {
		return true
	}
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		if ru.NeedsLibraryFacts || ru.JavaFacts != nil {
			return true
		}
	}
	return false
}

func moduleOnlyRules(activeRules []*api.Rule) []*api.Rule {
	out := make([]*api.Rule, 0, len(activeRules))
	for _, ru := range activeRules {
		if ru != nil && ru.Needs.Has(api.NeedsModuleIndex) {
			out = append(out, ru)
		}
	}
	return out
}

// classifyCrossFileNeeds inspects activeRules and reports whether any
// require an index-backed cross-file pass, a parsed-files pass, or a
// module-aware pass. Pure helper so the cross-file gating can be
// unit-tested in isolation.
func classifyCrossFileNeeds(activeRules []*api.Rule) (hasIndexBacked, hasParsedFiles, hasModuleAware bool) {
	for _, ru := range activeRules {
		if ru == nil {
			continue
		}
		if ru.Needs.Has(api.NeedsParsedFiles) {
			hasParsedFiles = true
			continue
		}
		if ru.Needs.Has(api.NeedsCrossFile) {
			hasIndexBacked = true
		}
	}
	for _, ru := range activeRules {
		if ru != nil && ru.Needs.Has(api.NeedsModuleIndex) {
			hasModuleAware = true
			break
		}
	}
	return
}

func cachedHashesOrNil(result *cache.Result) map[string]string {
	if result == nil || len(result.CachedHashes) == 0 {
		return nil
	}
	return result.CachedHashes
}
