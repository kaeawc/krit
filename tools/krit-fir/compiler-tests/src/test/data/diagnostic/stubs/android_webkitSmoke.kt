// Smoke: WebView settings, a WebViewClient subclass, and a JS bridge.
package stubs

import android.annotation.SuppressLint
import android.net.http.SslError
import android.webkit.JavascriptInterface
import android.webkit.SslErrorHandler
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient

class SmokeBridge {
    @JavascriptInterface
    fun postMessage(message: String) {
        println(message)
    }
}

class SmokeClient : WebViewClient() {
    override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean =
        request.url.scheme != "https"

    override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
        handler.cancel()
    }
}

@SuppressLint("SetJavaScriptEnabled")
fun configure(webView: WebView) {
    val settings: WebSettings = webView.settings
    settings.javaScriptEnabled = true
    settings.domStorageEnabled = true
    settings.allowFileAccess = false
    settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
    webView.webViewClient = SmokeClient()
    webView.addJavascriptInterface(SmokeBridge(), "bridge")
    webView.evaluateJavascript("1 + 1") { result -> println(result) }
    webView.loadUrl("https://example.com")
}
