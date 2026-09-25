// Smoke: the Apache HttpClient interface as an injectable dependency.
package stubs

import org.apache.http.HttpResponse
import org.apache.http.client.HttpClient
import org.apache.http.client.methods.HttpUriRequest

class ApacheGateway(private val client: HttpClient) {
    fun send(request: HttpUriRequest): HttpResponse = client.execute(request)
}
