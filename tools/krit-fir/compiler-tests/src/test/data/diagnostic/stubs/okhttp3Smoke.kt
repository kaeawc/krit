// Smoke: OkHttpClient.Builder with interceptors and TLS config (JDK javax.net.ssl
// types), request building, and sync/async calls.
package stubs

import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager
import okhttp3.Call
import okhttp3.Callback
import okhttp3.CertificatePinner
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

fun buildClient(factory: SSLSocketFactory, trustManager: X509TrustManager): OkHttpClient =
    OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .addInterceptor(Interceptor { chain -> chain.proceed(chain.request()) })
        .addNetworkInterceptor { chain -> chain.proceed(chain.request().newBuilder().header("X", "1").build()) }
        .hostnameVerifier(HostnameVerifier { _, _ -> true })
        .sslSocketFactory(factory, trustManager)
        .certificatePinner(CertificatePinner.Builder().add("example.com", "sha256/AAAA").build())
        .build()

fun execute(client: OkHttpClient) {
    val request = Request.Builder()
        .url("https://example.com")
        .addHeader("Accept", "application/json")
        .post("{}".toRequestBody("application/json".toMediaType()))
        .build()
    client.newCall(request).execute().use { response: Response ->
        if (response.isSuccessful) println(response.body?.string() + response.code)
    }
    client.newCall(Request.Builder().url("https://example.com").get().build()).enqueue(object : Callback {
        override fun onFailure(call: Call, e: IOException) {}

        override fun onResponse(call: Call, response: Response) {
            response.close()
        }
    })
    println(client.newBuilder().build().hostnameVerifier)
}
