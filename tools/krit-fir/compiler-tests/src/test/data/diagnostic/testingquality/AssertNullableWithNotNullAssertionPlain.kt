// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The rule runs on test-support files only: this file name is not a test file name.
package test.assertnullablewithnotnullassertion.plain

import org.junit.Assert.assertEquals

fun production(maybe: String?) {
    assertEquals("x", maybe!!)
}
