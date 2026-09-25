// Smoke: HttpGet/HttpPost are HttpUriRequests.
package stubs

import org.apache.http.client.methods.HttpGet
import org.apache.http.client.methods.HttpPost
import org.apache.http.client.methods.HttpUriRequest

val ApacheRequests: List<HttpUriRequest> = listOf(HttpGet("https://example.com"), HttpPost("https://example.com"))
