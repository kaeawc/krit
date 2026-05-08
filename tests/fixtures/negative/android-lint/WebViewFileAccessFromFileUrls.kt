package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class BrowserActivity {
    fun setupWebView(webView: WebView) {
        webView.settings.allowFileAccessFromFileURLs = false
        webView.settings.setAllowFileAccessFromFileURLs(false)
    }
}
