// Compiler-test source stubs; never packaged in the production artifact.
package org.junit.jupiter.api.function

fun interface Executable {
    @Throws(Throwable::class)
    fun execute()
}
