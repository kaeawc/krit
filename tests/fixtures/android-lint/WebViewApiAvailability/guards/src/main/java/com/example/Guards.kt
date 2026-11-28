package com.example

import android.content.Context
import android.os.Build
import android.webkit.WebView

fun guards(webView: WebView, context: Context, x: Boolean) {
    if (!(Build.VERSION.SDK_INT < 26)) {
        webView.getWebChromeClient()
    }
    if (Build.VERSION.SDK_INT < 26 || x) {
    } else {
        webView.getWebViewClient()
    }
    when {
        Build.VERSION.SDK_INT >= 26 -> webView.getWebChromeClient()
        else -> {}
    }
    if (Build.VERSION.SDK_INT >= 27) {
        WebView.startSafeBrowsing(context) { ok -> println(ok) }
    }
}
