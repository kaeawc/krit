package com.example

import android.webkit.WebView

class WebBridge {
    fun bind(webView: WebView) {
        webView.addJavascriptInterface(Any(), "bridge")
    }
}
