// Smoke: string and multi-value @ValueSource arrays.
package stubs

import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource

class ProviderSmoke {
    @ParameterizedTest
    @ValueSource(strings = ["", " "])
    fun blank(value: String) {
        println(value.isBlank())
    }

    @ParameterizedTest
    @ValueSource(longs = [1L], booleans = [true])
    fun mixed(value: Long) {
        println(value)
    }
}
