// Compiler-test source stubs; never packaged in the production artifact.
package org.apache.http.client.methods

interface HttpUriRequest {
    val method: String

    fun abort()
}

open class HttpGet(uri: String) : HttpUriRequest {
    override val method: String
        get() = TODO()

    override fun abort() {
        TODO()
    }
}

open class HttpPost(uri: String) : HttpUriRequest {
    override val method: String
        get() = TODO()

    override fun abort() {
        TODO()
    }
}
