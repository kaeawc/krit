package fixtures.negative.potentialbugs

import java.io.IOException

class UnreachableCatchBlock {
    fun example() {
        try {
            riskyOperation()
        } catch (e: IOException) {
            println("caught io exception")
        } catch (e: Exception) {
            println("caught exception")
        }
    }

    private fun riskyOperation() {
        throw IOException("fail")
    }
}
