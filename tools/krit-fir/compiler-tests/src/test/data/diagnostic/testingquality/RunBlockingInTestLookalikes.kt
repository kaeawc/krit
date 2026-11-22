// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 19
package test.runblocking.lookalikes

private annotation class Test
private fun runBlocking(block: () -> Unit) { block() }

class BlockingLookalikeTests {
    // Go reports by spelling; FIR proves both symbols are project lookalikes and stays silent.
    @Test
    fun bothLookalikes() = runBlocking { assert(true) } // FIR abstains: both resolved symbols are project lookalikes.

    // Go reports by Test's simple name; this annotation is not a supported test annotation.
    @Test
    fun annotationLookalike() = kotlinx.coroutines.runBlocking { assert(true) } // FIR abstains: @Test is not a supported test class.

    // Go reports by the callee's simple name; this is not kotlinx.coroutines.runBlocking.
    @org.junit.Test
    fun callLookalike() = runBlocking { assert(true) } // FIR abstains: runBlocking resolves to a project function.
}
