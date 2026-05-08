package test

import okhttp3.OkHttpClient
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

class ClientFactory {
    fun defaultClient(): OkHttpClient {
        return OkHttpClient.Builder().build()
    }

    fun verifier(): OkHttpClient {
        return OkHttpClient.Builder()
            .hostnameVerifier { host, session -> host == session.peerHost }
            .build()
    }

    fun trustManager(socketFactory: SSLSocketFactory, validatingManager: X509TrustManager): OkHttpClient {
        return OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, validatingManager)
            .build()
    }
}
