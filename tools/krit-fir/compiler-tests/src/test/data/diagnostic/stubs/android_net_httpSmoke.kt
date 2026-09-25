// Smoke: the removed AndroidHttpClient and the SslError passed to WebViewClient.
package stubs

import android.net.http.AndroidHttpClient
import android.net.http.SslError
import org.apache.http.HttpResponse
import org.apache.http.client.methods.HttpGet

@Suppress("DEPRECATION")
fun legacyFetch(): HttpResponse {
    val client = AndroidHttpClient.newInstance("smoke")
    try {
        return client.execute(HttpGet("https://example.com"))
    } finally {
        client.close()
    }
}

fun untrusted(error: SslError): Boolean =
    error.primaryError == SslError.SSL_UNTRUSTED || error.hasError(SslError.SSL_EXPIRED)
