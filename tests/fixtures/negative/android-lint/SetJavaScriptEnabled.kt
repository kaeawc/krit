package com.example

import android.webkit.WebView

class BrowserActivity {
    fun setupWebView(webView: WebView) {
        webView.settings.javaScriptEnabled = false
    }
}
