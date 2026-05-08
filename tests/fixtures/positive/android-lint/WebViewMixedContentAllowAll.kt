package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
        webView.settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW)
    }
}
