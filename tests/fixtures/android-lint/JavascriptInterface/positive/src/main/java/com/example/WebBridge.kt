package com.example

import android.webkit.WebView

class Bridge {
    fun unannotated() = Unit
}

class WebBridge {
    fun bind(webView: WebView) {
        webView.addJavascriptInterface(Bridge(), "bridge")
    }
}
