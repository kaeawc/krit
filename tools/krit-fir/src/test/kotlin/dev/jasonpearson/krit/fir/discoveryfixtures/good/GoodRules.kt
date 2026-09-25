package dev.jasonpearson.krit.fir.discoveryfixtures.good

import dev.jasonpearson.krit.fir.FirRule

// Discovery fixtures, scanned with an explicit prefix by FirRuleDiscoveryTest.
// They live outside checkers/ so the live rule set never sees them.

/** A file-private top-level object compiles to a package-private JVM class. */
private object PrivateObjectRule : FirRule {
    override val ruleId = "DiscoveryPrivateObject"
}

object DiscoveryHolder {
    object NestedObjectRule : FirRule {
        override val ruleId = "DiscoveryNestedObject"
    }
}

/** Abstract bases and interfaces are legitimately not instantiated. */
abstract class AbstractRuleBase : FirRule

interface RuleMarker : FirRule
