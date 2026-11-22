// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.runblocking.aliases

import kotlinx.coroutines.runBlocking as blockingBridge
import org.junit.Test as UnitTest

typealias BlockingTestAlias = org.junit.jupiter.api.Test

class BlockingAliasTests {
    // Go's name heuristic is silent; confirm-only drops any FIR-only alias finding.
    @org.junit.Test
    fun callAlias() = blockingBridge { assert(true) }

    // Go's annotation-name heuristic is silent; confirm-only drops any FIR-only alias finding.
    @UnitTest
    fun annotationAlias() = kotlinx.coroutines.runBlocking { assert(true) }

    // Both spellings are aliases; confirm-only rejects any finding Go cannot confirm.
    @UnitTest
    fun bothAliases() = blockingBridge { assert(true) }

    // Go's annotation-name heuristic is silent; confirm-only drops any FIR-only alias finding.
    @BlockingTestAlias
    fun annotationTypeAlias() = kotlinx.coroutines.runBlocking { assert(true) }
}
