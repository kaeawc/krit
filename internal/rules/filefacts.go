package rules

import (
	"sync/atomic"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/librarymodel"
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
	slotClassLikeDeclsByName        = "classLikeDeclsByName"
	slotLintSourceFileClasses       = "lintSourceFileClasses"
	slotJumpMetrics                 = "jumpmetrics"
	slotScopeReassignments          = "scopeReassignments"
	slotRangeSummary                = "rangesummary"
	slotAddJSInterfaceSDK           = "addjsinterface_sdk"
	slotDoubleMutabilityShadows     = "doubleMutabilityShadows"
	slotDeprecatedDecls             = "deprecatedDecls"
	slotPrecompileDeclSites         = "precompileDeclSites"
	slotNullSafety                  = "nullsafety"
	slotManifestGradle              = "manifestGradle"
	slotManifestDOM                 = "manifestDOM"
	slotVersionCatalogPlugins       = "versionCatalogPlugins"
	slotLintCatalogPlugins          = "lintCatalogPlugins"
	slotSourceModuleGradle          = "sourceModuleGradle"
	slotTestQualityMockHelpers      = "testQualityMockHelpers"
	slotTestQualityMockNames        = "testQualityMockNames"
	slotTestQualityAssertionHelpers = "testQualityAssertionHelpers"
	slotUnusedImportRefs            = "unusedImportRefs"
	slotUnusedVariableHasParseError = "unusedVariableHasParseError"
	slotPrintBuiltinShadows         = "printBuiltinShadows"
	slotComposeContentDescImports   = "composeContentDescImports"
	slotDeclaredTypeNames           = "declaredTypeNames"
	slotPrngCryptoFile              = "prngCryptoFile"
	slotSourceCommentRanges         = "sourceCommentRanges"
	slotUnnecessaryFQNImports       = "unnecessaryFQNImports"
	slotFunctionNames               = "functionNames"
	slotDeclaredFunctionNames       = "declaredFunctionNames"
	slotCallExpressionsByName       = "callExpressionsByName"
	slotCatalogDiscovery            = "catalogDiscovery"
)

// catalogDiscoveryKey is the RequestFact key of the catalog discovery memo, which
// lives for one request: a project run, or one call of a long-lived analyzer
// (see Dispatcher.BeginRequest).
const catalogDiscoveryKey = "request"

// loadGradleVersionCatalog loads the version catalog for a Gradle file, sharing
// one catalog-source discovery memo across the request's rules and files. facts
// is the dispatcher's cache from api.Context.Facts, not the process-wide
// fileFactsCache(), which belongs to whichever dispatcher was built last.
func loadGradleVersionCatalog(facts *filefacts.Cache, gradlePath string) (librarymodel.VersionCatalog, bool) {
	return catalogDiscovery(facts).LoadVersionCatalogForGradlePath(gradlePath)
}

// catalogDiscovery returns the request's catalog discovery memo, or nil (no
// memoization) without a cache.
func catalogDiscovery(facts *filefacts.Cache) *librarymodel.CatalogDiscovery {
	if facts == nil {
		return nil
	}
	return filefacts.RequestFact(facts, catalogDiscoveryKey, slotCatalogDiscovery, librarymodel.NewCatalogDiscovery)
}
