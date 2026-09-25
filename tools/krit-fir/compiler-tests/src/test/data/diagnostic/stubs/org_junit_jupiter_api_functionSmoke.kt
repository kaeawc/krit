// Smoke: Executable is a throwing SAM for Assertions.assertThrows.
package stubs

import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.function.Executable

val Failing: Executable = Executable { error("boom") }

fun runFailing() {
    Assertions.assertThrows(IllegalStateException::class.java, Failing)
    Failing.execute()
}
