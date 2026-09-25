// Smoke: Truth.assertThat overloads pick the specialized Subject types.
package stubs

import com.google.common.truth.Truth
import com.google.common.truth.Truth.assertThat

fun truthSmoke(nothing: Any?) {
    assertThat(1 + 1).isEqualTo(2)
    assertThat(3).isGreaterThan(2)
    assertThat("abc").startsWith("a")
    assertThat("abc").contains("b")
    assertThat(listOf(1, 2)).containsExactly(1, 2).inOrder()
    assertThat(listOf(1)).hasSize(1)
    assertThat(true).isTrue()
    assertThat(nothing).isNull()
    Truth.assertThat(nothing).isNotEqualTo(1)
    Truth.assertWithMessage("context").that(nothing).isNull()
}
