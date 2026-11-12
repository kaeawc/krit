// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 20, 24, 29, 35, 48
// A `!!` on a nullable operand in the arguments of a JUnit assertion.
package test.assertnullablewithnotnullassertion

import org.junit.Assert
import org.junit.Assert.assertEquals
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
