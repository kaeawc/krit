package rules

import (
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRulesWithTypeInfoDeclareExplicitJavaSupport is the lint gate
// referenced by issue #194: every rule that declares NeedsTypeInfo
// (i.e. NeedsResolver) must carry an explicit Java LanguageSupport
// entry. Type-aware rules are exactly the ones where Java parity is
// most likely to silently drift, so we force the author to record
// whether Java is supported / partial / pending / not-applicable /
// needs-design rather than silently inheriting a ruleset default.
func TestRulesWithTypeInfoDeclareExplicitJavaSupport(t *testing.T) {
	matrix := JavaSupportReadiness()
	var missing []string
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		if !r.Needs.Has(api.NeedsResolver) {
			continue
		}
		if _, ok := matrix.Rules[r.ID]; ok {
			continue
		}
		// Per-rule LanguageSupport on the Rule literal also counts as
		// an explicit declaration.
		if _, ok := r.LanguageSupport[JavaLanguageSupportKey]; ok {
			continue
		}
		if grandfatheredTypeInfoRulesWithoutExplicitJavaSupport[r.ID] {
			continue
		}
		missing = append(missing, r.ID+" (ruleset "+r.Category+")")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("rules with NeedsTypeInfo/NeedsResolver must declare an explicit Java LanguageSupport entry in JavaSupportReadiness.Rules or Rule.LanguageSupport:\n  %s",
			strings.Join(missing, "\n  "))
	}

	// Guard against staleness: a grandfathered rule that loses NeedsResolver
	// or gains an explicit entry no longer belongs on the allowlist.
	var stale []string
	for id := range grandfatheredTypeInfoRulesWithoutExplicitJavaSupport {
		r := findRegistryRule(id)
		if r == nil {
			stale = append(stale, id+" (no longer registered)")
			continue
		}
		if !r.Needs.Has(api.NeedsResolver) {
			stale = append(stale, id+" (no longer needs resolver)")
			continue
		}
		if _, ok := matrix.Rules[id]; ok {
			stale = append(stale, id+" (now classified in JavaSupportReadiness)")
			continue
		}
		if _, ok := r.LanguageSupport[JavaLanguageSupportKey]; ok {
			stale = append(stale, id+" (now has explicit LanguageSupport)")
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("grandfathered list is stale; remove these from grandfatheredTypeInfoRulesWithoutExplicitJavaSupport:\n  %s",
			strings.Join(stale, "\n  "))
	}
}

func findRegistryRule(id string) *api.Rule {
	for _, r := range api.Registry {
		if r != nil && r.ID == id {
			return r
		}
	}
	return nil
}

// grandfatheredTypeInfoRulesWithoutExplicitJavaSupport lists pre-existing
// type-aware rules that have not yet been classified per-rule for Java
// support. The lint gate above forbids adding *new* NeedsTypeInfo rules
// without an explicit entry; this set should only shrink as rules either
// gain entries in JavaSupportReadiness.Rules or have their type-info
// requirement dropped.
var grandfatheredTypeInfoRulesWithoutExplicitJavaSupport = map[string]bool{
	"AbstractClassCanBeConcreteClass":                true,
	"AbstractMemberNotImplemented":                   true,
	"AnalyticsCallWithoutConsentGate":                true,
	"AnalyticsEventWithPiiParamName":                 true,
	"AnalyticsUserIdFromPii":                         true,
	"AnimatorDurationIgnoresScale":                   true,
	"ArrayPrimitive":                                 true,
	"AvoidReferentialEquality":                       true,
	"CanBeNonNullable":                               true,
	"CastNullableToNonNullableType":                  true,
	"CharArrayToStringCall":                          true,
	"ContentProviderQueryWithSelectionInterpolation": true,
	"CouldBeSequence":                                true,
	"CrashlyticsCustomKeyWithPii":                    true,
	"DataClassShouldBeImmutable":                     true,
	"Deprecation":                                    true,
	"DontDowncastCollectionTypes":                    true,
	"ElseCaseInsteadOfExhaustiveWhen":                true,
	"ErrorUsageWithThrowable":                        true,
	"ExplicitCollectionElementAccessMethod":          true,
	"FirebaseRemoteConfigDefaultsWithPii":            true,
	"ForbiddenMethodCall":                            true,
	"IgnoredReturnValue":                             true,
	"ImageLoadedAtFullSizeInList":                    true,
	"ImplicitDefaultLocale":                          true,
	"InlinedApi":                                     true,
	"IteratorHasNextCallsNextMethod":                 true,
	"IteratorNotThrowingNoSuchElementException":      true,
	"LogConditional":                                 true,
	"LogOfSharedPreferenceRead":                      true,
	"MainDispatcherInLibraryCode":                    true,
	"MapGetWithNotNullAssertionOperator":             true,
	"MissingPermission":                              true,
	"MissingReturn":                                  true,
	"MissingUseCall":                                 true,
	"NewApi":                                         true,
	"NoElseInWhenSealed":                             true,
	"NonExhaustiveWhen":                              true,
	"NullCheckOnMutableProperty":                     true,
	"NullableToStringCall":                           true,
	"ObjectAnimatorBinding":                          true,
	"ObjectExtendsThrowable":                         true,
	"ObjectLiteralToLambda":                          true,
	"ObsoleteLayoutParam":                            true,
	"OverrideSignatureMismatch":                      true,
	"PlainFileWriteOfSensitive":                      true,
	"ProtectedMemberInFinalClass":                    true,
	"Range":                                          true,
	"RedundantExplicitType":                          true,
	"RedundantSuspendModifier":                       true,
	"Registered":                                     true,
	"RequiresApiViolation":                           true,
	"RoomRawQueryStringConcat":                       true,
	"SerialVersionUIDInSerializableClass":            true,
	"SharedPreferencesForSensitiveKey":               true,
	"SuspendFunSwallowedCancellation":                true,
	"TestFixtureAccessedFromProduction":              true,
	"TimberTreeNotPlanted":                           true,
	"TooGenericExceptionThrown":                      true,
	"UnnecessaryFilter":                              true,
	"UnnecessaryNotNullCheck":                        true,
	"UnnecessaryNotNullOperator":                     true,
	"UnnecessarySafeCall":                            true,
	"UnnecessaryTypeCasting":                         true,
	"UnreachableCatchBlock":                          true,
	"UnsafeCast":                                     true,
	"UseCheckNotNull":                                true,
	"UseIsNullOrEmpty":                               true,
	"UseRequireNotNull":                              true,
	"UselessCallOnNotNull":                           true,
	"UselessElvisOnNonNull":                          true,
	"ViewHolder":                                     true,
	"ViewTag":                                        true,
	"Wakelock":                                       true,
	"WithContextInSuspendFunctionNoop":               true,
	"WrongConstant":                                  true,
}
