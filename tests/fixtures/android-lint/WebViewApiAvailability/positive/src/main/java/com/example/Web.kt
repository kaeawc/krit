package com.example

import android.content.Context
import android.os.Build
import android.webkit.WebView

fun clients(webView: WebView, context: Context) {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
        webView.getWebChromeClient()
        WebView.getCurrentWebViewPackage()
    }
    if (Build.VERSION.SDK_INT >= 27) {
        WebView.startSafeBrowsing(context, null)
    }
    if (Build.VERSION.SDK_INT < 23) return
    webView.createWebMessageChannel()
}
