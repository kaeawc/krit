// Smoke: the deprecated Apache DefaultHttpClient executing an HttpGet.
package stubs

import org.apache.http.HttpResponse
import org.apache.http.client.HttpClient
import org.apache.http.client.methods.HttpGet
import org.apache.http.impl.client.DefaultHttpClient

@Suppress("DEPRECATION")
fun apacheFetch(): HttpResponse {
    val client: HttpClient = DefaultHttpClient()
    return client.execute(HttpGet("https://example.com"))
}
