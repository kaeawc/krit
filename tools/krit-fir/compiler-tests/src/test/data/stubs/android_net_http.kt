// Compiler-test source stubs; never packaged in the production artifact.
package android.net.http

import android.content.Context
import org.apache.http.HttpResponse
import org.apache.http.client.HttpClient
import org.apache.http.client.methods.HttpUriRequest

// Removed in API 23; final class with static newInstance factories.
@Deprecated("Deprecated in Java")
object AndroidHttpClient : HttpClient {
    fun newInstance(userAgent: String): AndroidHttpClient = TODO()

    fun newInstance(userAgent: String, context: Context?): AndroidHttpClient = TODO()

    override fun execute(request: HttpUriRequest): HttpResponse = TODO()

    fun close() {
        TODO()
    }
}

open class SslError {
    open val primaryError: Int
        get() = TODO()

    open val url: String
        get() = TODO()

    open fun hasError(error: Int): Boolean = TODO()

    companion object {
        const val SSL_NOTYETVALID: Int = 0
        const val SSL_EXPIRED: Int = 1
        const val SSL_IDMISMATCH: Int = 2
        const val SSL_UNTRUSTED: Int = 3
        const val SSL_DATE_INVALID: Int = 4
        const val SSL_INVALID: Int = 5
    }
}
