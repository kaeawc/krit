package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowContentAccess = true
        webView.settings.setAllowContentAccess(true)
    }
}
