package test

import okhttp3.OkHttpClient
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

class ClientFactory {
    fun verifier(): OkHttpClient {
        return OkHttpClient.Builder()
            .hostnameVerifier { _, _ -> true }
            .build()
    }

    fun trustManager(socketFactory: SSLSocketFactory, unsafeTrustManager: X509TrustManager): OkHttpClient {
        return OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, unsafeTrustManager)
            .build()
    }
}
