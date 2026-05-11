package scan

import (
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/pipeline"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func shouldOpenResourceIndexCache(activeRules []*api.Rule, noResourceCache bool) bool {
	if noResourceCache || !pipeline.RulesNeedAndroidProject(activeRules) {
		return false
	}
	return true
}

func canParseOnlyCacheMisses(activeRules []*api.Rule, cacheResult *cache.Result, useCache bool, allowCrossFileDelta bool, allowResourceSourceDelta bool) bool {
	return pipeline.CanParseOnlyCacheMisses(activeRules, cacheResult, useCache, allowCrossFileDelta, allowResourceSourceDelta)
}

func cacheMissPaths(paths []string, cacheResult *cache.Result) []string {
	return pipeline.CacheMissPaths(paths, cacheResult)
}

func rulesNeedAndroidProject(activeRules []*api.Rule) bool {
	return pipeline.RulesNeedAndroidProject(activeRules)
}

func rulesNeedProjectModel(activeRules []*api.Rule) bool {
	return pipeline.RulesNeedProjectModel(activeRules)
}

func classifyCrossFileNeeds(activeRules []*api.Rule) (hasIndexBacked, hasParsedFiles, hasModuleAware bool) {
	return pipeline.ClassifyCrossFileNeeds(activeRules)
}
