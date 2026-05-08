package com.example

import android.webkit.WebView

class MyWebView {
    fun setup(webView: WebView) {
        webView.addJavascriptInterface(JsBridge(), "bridge")
    }
}
