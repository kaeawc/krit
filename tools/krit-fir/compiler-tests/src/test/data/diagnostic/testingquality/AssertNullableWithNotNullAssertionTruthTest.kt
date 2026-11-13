// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 13
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
