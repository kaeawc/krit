// Compiler-test source stubs; never packaged in the production artifact.
package org.apache.http

interface HttpResponse {
    val statusLine: StatusLine

    val entity: HttpEntity?
}

interface StatusLine {
    val statusCode: Int

    val reasonPhrase: String?
}

interface HttpEntity {
    val contentLength: Long
}
