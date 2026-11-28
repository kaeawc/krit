package com.example

import android.os.Build
import android.webkit.WebView

class FakeWebView {
    fun getWebChromeClient(): Any? = null
}

fun clients(webView: WebView, fake: FakeWebView) {
    if (Build.VERSION.SDK_INT >= 23) {
        webView.getWebChromeClient()
    }
    if (Build.VERSION.SDK_INT >= 26) {
        fake.getWebChromeClient()
        webView.reload()
        webView.setDataDirectorySuffix("x")
    }
    if (Build.VERSION.SDK_INT >= 28) {
        webView.settings
    }
}
