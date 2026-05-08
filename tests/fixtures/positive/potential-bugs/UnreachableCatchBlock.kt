package fixtures.positive.potentialbugs

import java.io.IOException

class UnreachableCatchBlock {
    fun example() {
        try {
            riskyOperation()
        } catch (e: Exception) {
            println("caught exception")
        } catch (e: IOException) {
            println("unreachable")
        }
    }

    private fun riskyOperation() {
        throw IOException("fail")
    }
}
