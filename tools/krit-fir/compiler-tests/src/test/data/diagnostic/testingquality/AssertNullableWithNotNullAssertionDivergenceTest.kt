// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes Go and the compiler read differently.
package test.assertnullablewithnotnullassertion.divergence

import org.junit.Assert.assertEquals as expectEqual
import org.junit.Assert.assertEquals

fun smartCast(maybe: String?) {
    if (maybe != null) {
        // The compiler smart-casts maybe to String, so the `!!` is unnecessary; there is nothing to check first.
        assertEquals("x", maybe!!)
    }
}

fun importAlias(maybe: String?) {
    // Go reads the written name expectEqual, which is not an assertion name.
    expectEqual("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}
