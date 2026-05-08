package com.example.app.utils

import android.webkit.WebView
import java.net.HttpURLConnection
import java.net.URL
import java.security.SecureRandom
import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager

// Security issues galore
class NetworkHelper {

    // AddJavascriptInterface
    fun setupWebView(webView: WebView) {
        webView.settings.javaScriptEnabled = true
        webView.addJavascriptInterface(JsBridge(), "android")
    }

    // TrustAllCerts: disables SSL certificate validation
    fun disableSslVerification() {
        val trustAllCerts = arrayOf<TrustManager>(object : X509TrustManager {
            override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {}
            override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {}
            override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
        })

        val sslContext = SSLContext.getInstance("TLS")
        sslContext.init(null, trustAllCerts, SecureRandom())
        HttpsURLConnection.setDefaultSSLSocketFactory(sslContext.socketFactory)

        val allHostsValid = HostnameVerifier { _, _ -> true }
        HttpsURLConnection.setDefaultHostnameVerifier(allHostsValid)
    }

    // SdCardPath: hardcoded SD card path
    fun getCachePath(): String {
        return "/sdcard/myapp/cache"
    }

    // WorldReadableFiles
    fun writeConfig(data: String) {
        val file = java.io.File("/data/data/com.example.app/shared_prefs/config.xml")
        file.setReadable(true, false)
        file.writeText(data)
    }

    // MagicNumber + hardcoded URL
    fun fetchData(endpoint: String): String {
        val url = URL("http://api.example.com:8080/$endpoint")
        val connection = url.openConnection() as HttpURLConnection
        connection.connectTimeout = 5000
        connection.readTimeout = 10000
        connection.requestMethod = "GET"

        return try {
            val responseCode = connection.responseCode
            if (responseCode == 200) {
                connection.inputStream.bufferedReader().readText()
            } else {
                ""
            }
        } finally {
            connection.disconnect()
        }
    }

    // SetJavaScriptEnabled
    fun configureWebView(webView: WebView) {
        webView.settings.javaScriptEnabled = true
        webView.settings.allowFileAccess = true
        webView.settings.allowContentAccess = true
    }

    // EmptyFunctionBlock
    fun onNetworkChanged() {
    }

    class JsBridge {
        fun showToast(message: String) {
            // bridge method
        }
    }
}
