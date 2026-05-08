package test

import org.junit.Assert.assertEquals
import com.google.common.truth.Truth.assertThat

fun testCompute() {
    val actual = compute()
    assertEquals(42, actual)
    assertThat(actual).isEqualTo(42)
}
