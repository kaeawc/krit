package fixtures.negative.potentialbugs

import java.io.IOException
import java.net.SocketException
import java.net.SocketTimeoutException

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

    // SocketTimeoutException extends InterruptedIOException, not
    // SocketException, so the second clause is reachable.
    fun socketTimeout() {
        try {
            riskyOperation()
        } catch (e: SocketException) {
            println("caught socket exception")
        } catch (e: SocketTimeoutException) {
            println("caught timeout")
        }
    }

    private fun riskyOperation() {
        throw IOException("fail")
    }
}
