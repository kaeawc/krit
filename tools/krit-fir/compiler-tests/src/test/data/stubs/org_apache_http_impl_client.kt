// Compiler-test source stubs; never packaged in the production artifact.
package org.apache.http.impl.client

import org.apache.http.HttpResponse
import org.apache.http.client.HttpClient
import org.apache.http.client.methods.HttpUriRequest

// Real chain: DefaultHttpClient -> AbstractHttpClient -> CloseableHttpClient.
@Deprecated("Deprecated in Java")
abstract class CloseableHttpClient : HttpClient, java.io.Closeable

@Deprecated("Deprecated in Java")
abstract class AbstractHttpClient : CloseableHttpClient()

@Deprecated("Deprecated in Java")
open class DefaultHttpClient : AbstractHttpClient() {
    override fun execute(request: HttpUriRequest): HttpResponse = TODO()

    override fun close() {
        TODO()
    }
}
