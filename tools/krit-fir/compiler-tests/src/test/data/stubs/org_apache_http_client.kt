// Compiler-test source stubs; never packaged in the production artifact.
package org.apache.http.client

import org.apache.http.HttpResponse
import org.apache.http.client.methods.HttpUriRequest

interface HttpClient {
    fun execute(request: HttpUriRequest): HttpResponse
}
