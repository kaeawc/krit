package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowUniversalAccessFromFileURLs = true
        webView.settings.setAllowUniversalAccessFromFileURLs(true)
    }
}
