package org.junit.rules

class ExpectedException {
    fun expect(type: Class<out Throwable>) = Unit
    companion object {
        fun none(): ExpectedException = TODO()
    }
}

class ErrorCollector {
    fun <T> checkThat(actual: T, matcher: Any) = Unit
}
