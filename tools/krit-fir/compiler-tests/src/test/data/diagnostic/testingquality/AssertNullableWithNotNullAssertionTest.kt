// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 21, 25, 30, 36, 49, 55, 60, 67, 72
// A `!!` on a nullable operand in the arguments of a JUnit assertion.
package test.assertnullablewithnotnullassertion

import org.junit.Assert
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue

class Holder {
    var field: String? = null
    val items: List<String>? = null
}

fun direct(maybe: String?) {
    assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun qualified(maybe: String?) {
    Assert.assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun memberReceiver(holder: Holder?) {
    assertEquals(1, holder<!AssertNullableWithNotNullAssertion!>!!<!>.items?.size)
}

fun twoOperands(a: String?, b: String?) {
    // One finding per call, on the first `!!`.
    assertEquals(a<!AssertNullableWithNotNullAssertion!>!!<!>, b!!)
}

fun multiLine(maybe: String?) {
    assertTrue(
        "message",
        maybe<!AssertNullableWithNotNullAssertion!>!!<!>.isEmpty(),
    )
}

// An unstable property is not smart-cast, so the `!!` is required. Go misses this: it cannot type holder.field,
// a property of a parameter.
fun unstableProperty(holder: Holder) {
    if (holder.field != null) {
        assertEquals("x", holder.field<!AssertNullableWithNotNullAssertion!>!!<!>)
    }
}

fun comparison(maybe: String?) {
    assertTrue(maybe<!AssertNullableWithNotNullAssertion!>!!<!> == "x")
}

// A check in a comment, a string, another branch or a lambda does not run before the assertion.
fun checkInComment(maybe: String?) {
    // assertNotNull(maybe)
    assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun checkInString(maybe: String?) {
    println("assertNotNull(maybe)")
    assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun checkInOtherBranch(maybe: String?, flag: Boolean) {
    if (flag) {
        assertNotNull(maybe)
    }
    assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun checkInLambda(maybe: String?) {
    run { assertNotNull(maybe) }
    assertEquals("x", maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}
