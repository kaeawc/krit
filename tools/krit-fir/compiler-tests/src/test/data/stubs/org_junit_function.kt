// Compiler-test source stubs; never packaged in the production artifact.
package org.junit.function

fun interface ThrowingRunnable {
    @Throws(Throwable::class)
    fun run()
}
