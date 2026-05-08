package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
        webView.settings.setMixedContentMode(WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE)
    }
}
