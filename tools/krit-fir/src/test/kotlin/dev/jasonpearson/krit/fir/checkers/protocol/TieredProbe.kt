package dev.jasonpearson.krit.fir.checkers.protocol

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.WholeCompilationTier
import dev.jasonpearson.krit.fir.compilationFiles

/**
 * Test-only rule that is tiered while [armed]. No built-in rule is tiered any more, so the tests of the tiered-rule
 * policy and of the gate need one. It reports nothing and registers no checker. It lives under the test source set,
 * so only test classpaths discover it; it is unarmed by default, so a test that asks for "all rules" (and the bench)
 * does not meet a tiered rule unless it armed this one.
 */
object TieredProbe : FirRule {
    override val ruleId = "TieredProbe"

    @Volatile private var armedNow = false

    override val wholeCompilationTier: WholeCompilationTier?
        get() = if (armedNow) compilationFiles("test-only rule standing in for a rule that iterates the compilation's files") else null

    /** Runs [block] with the probe tiered, and unarms it afterwards. */
    fun armed(block: () -> Unit) {
        armedNow = true
        try {
            block()
        } finally {
            armedNow = false
        }
    }
}
