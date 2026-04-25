package com.example

import android.webkit.JavascriptInterface
import android.webkit.WebView

class Bridge {
    @JavascriptInterface
    fun exposed() = Unit
}

class WebBridge {
    fun bind(webView: WebView) {
        webView.addJavascriptInterface(Bridge(), "bridge")
    }
}
