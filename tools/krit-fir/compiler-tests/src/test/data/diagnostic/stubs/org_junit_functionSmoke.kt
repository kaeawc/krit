// Smoke: ThrowingRunnable is the SAM behind Assert.assertThrows.
package stubs

import org.junit.Assert
import org.junit.function.ThrowingRunnable

val Throwing: ThrowingRunnable = ThrowingRunnable { error("boom") }

fun assertThrowing(): IllegalStateException = Assert.assertThrows(IllegalStateException::class.java, Throwing)
