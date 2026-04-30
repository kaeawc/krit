package test

import org.junit.Test

class TestWithoutAssertionPositive {
    @Test
    fun loads() {
        val x = 42
        println(x)
    }

    @Test
    fun localExpectErrorLookalike() {
        expectError<IllegalStateException> { parse("") }
    }

    @Test
    fun uncheckedLookalike() {
        uncheckedTemplateFormat()
    }

    private fun <T : Throwable> expectError(block: () -> Unit) {
        block()
    }

    private fun uncheckedTemplateFormat() {
        println("not a check helper")
    }
}
