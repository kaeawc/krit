package rules

import (
	"sync/atomic"

	"github.com/kaeawc/krit/internal/filefacts"
)

// sharedFileFacts is the run-scoped cache used by helpers that have a
// *scanner.File but no v2.Context to thread through. V2Dispatcher
// stores its cache here in NewV2Dispatcher, and helpers read it via
// fileFactsCache(). When no cache has been registered (unit tests,
// mini-contexts created outside the dispatcher), fileFactsCache returns
// nil and filefacts methods recompute without caching.
var sharedFileFacts atomic.Pointer[filefacts.Cache]

func setSharedFileFacts(c *filefacts.Cache) {
	sharedFileFacts.Store(c)
}

func fileFactsCache() *filefacts.Cache {
	return sharedFileFacts.Load()
}

// Slot names for per-file / per-node facts cached via filefacts.
// Defined as constants so callers cannot silently miss the cache via
// a string typo. Each value must be unique within the package.
const (
	slotComplexity                  = "complexity"
	slotFunctionDecl                = "fundecl"
	slotClassDecl                   = "classdecl"
	slotJumpMetrics                 = "jumpmetrics"
	slotScopeReassignments          = "scopeReassignments"
	slotRangeSummary                = "rangesummary"
	slotAddJSInterfaceSDK           = "addjsinterface_sdk"
	slotDoubleMutabilityShadows     = "doubleMutabilityShadows"
	slotDeprecatedDecls             = "deprecatedDecls"
	slotNullSafety                  = "nullsafety"
	slotManifestGradle              = "manifestGradle"
	slotSourceModuleGradle          = "sourceModuleGradle"
	slotTestQualityMockHelpers      = "testQualityMockHelpers"
	slotTestQualityMockNames        = "testQualityMockNames"
	slotTestQualityAssertionHelpers = "testQualityAssertionHelpers"
	slotUnusedImportRefs            = "unusedImportRefs"
	slotUnusedVariableHasParseError = "unusedVariableHasParseError"
	slotPrintBuiltinShadows         = "printBuiltinShadows"
)
