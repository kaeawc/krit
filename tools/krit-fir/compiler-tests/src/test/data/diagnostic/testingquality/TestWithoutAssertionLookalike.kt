// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
package test.testwithoutassertion.lookalike

annotation class Test
annotation class Ignore

class OrdinaryFunctions {
    // Go recognizes any annotation named Test; this declaration is not a test framework entry point.
    @Test
    fun ordinaryFunction() { println("work") } // FIR abstains: annotation does not resolve to a supported test framework.

    // Go exempts an Ignore annotation by its written name; FIR preserves that exemption.
    @Ignore
    @org.junit.Test
    fun ordinaryIgnoreAnnotation() { println("work") }
}
