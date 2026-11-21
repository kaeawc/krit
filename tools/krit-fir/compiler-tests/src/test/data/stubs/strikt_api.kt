package strikt.api

class Assertion<T> {
    fun isEqualTo(expected: T) = Unit
}
fun <T> expectThat(actual: T): Assertion<T> = TODO()
