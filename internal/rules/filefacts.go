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

// loadGradleVersionCatalog loads the version catalog for a Gradle file, sharing
// one catalog-source discovery memo across the run's rules and files.
func loadGradleVersionCatalog(gradlePath string) (librarymodel.VersionCatalog, bool) {
	return runCatalogDiscovery().LoadVersionCatalogForGradlePath(gradlePath)
}

// runCatalogDiscovery returns the run's catalog discovery memo, or nil (no
// memoization) outside a dispatcher run.
func runCatalogDiscovery() *librarymodel.CatalogDiscovery {
	c := fileFactsCache()
	if c == nil {
		return nil
	}
	return filefacts.StringFact(c, "run", slotCatalogDiscovery, librarymodel.NewCatalogDiscovery)
}
