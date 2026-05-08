package test

import com.google.common.truth.Truth.assertThat

fun testCompute() {
    val actual = compute()
    assertThat(actual).isEqualTo(42)
}
