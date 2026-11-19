// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 13, 37
// Truth assertions: `assertThat(x!!)` and the `isEqualTo(x!!)` argument are both subjects.
package test.assertnullablewithnotnullassertion.truth

import com.google.common.truth.Truth.assertThat

fun subject(maybe: String?) {
    assertThat(maybe<!AssertNullableWithNotNullAssertion!>!!<!>).isEqualTo("x")
}

fun expected(actual: String, maybe: String?) {
    assertThat(actual).isEqualTo(maybe<!AssertNullableWithNotNullAssertion!>!!<!>)
}

fun checkedFirst(maybe: String?) {
    assertThat(maybe).isNotNull()
    assertThat(maybe!!).isEqualTo("x")
}

fun nestedSubject(maybe: String?) {
    assertThat(maybe?.let { it!! }).isEqualTo("x")
}

class Box(val icon: String?)

// The check reads `box!!.icon`; Truth's isNotNull has no contract, so `box.icon!!` after it is required.
fun checkedThroughBang(box: Box?) {
    assertThat(box).isNotNull()
    assertThat(box!!.icon).isNotNull()
    assertThat(box.icon!!.length).isEqualTo(1)
}

// `box?.icon` checks the operand of `box!!.icon!!`; only `box` itself is unchecked.
fun checkedThroughSafeCall(box: Box?) {
    assertThat(box?.icon).isNotNull()
    assertThat(box<!AssertNullableWithNotNullAssertion!>!!<!>.icon!!.length).isEqualTo(1)
}
