package dev.jasonpearson.krit.fir.discoveryfixtures.bad

import dev.jasonpearson.krit.fir.FirRule

/** Author forgot `object`: discovery must fail loudly instead of dropping it. */
class NotAnObjectRule : FirRule {
    override val ruleId = "DiscoveryNotAnObject"
}
