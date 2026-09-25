// Smoke: @ParameterizedTest with its argument sources.
package stubs

import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import org.junit.jupiter.params.provider.MethodSource
import org.junit.jupiter.params.provider.ValueSource

class ParamsSmoke {
    @ParameterizedTest
    @ValueSource(ints = [1, 2, 3])
    fun positive(value: Int) {
        <!PrintlnInProduction!>println<!>(value)
    }

    @ParameterizedTest(name = "{0} has length {1}")
    @CsvSource("a, 1", "bb, 2")
    fun lengths(text: String, length: Int) {
        <!PrintlnInProduction!>println<!>(text.length == length)
    }

    @ParameterizedTest
    @MethodSource("names")
    fun fromMethod(name: String) {
        <!PrintlnInProduction!>println<!>(name)
    }

    companion object {
        @JvmStatic
        fun names(): List<String> = listOf("a", "b")
    }
}
