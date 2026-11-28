package com.example

import android.os.Build
import android.webkit.WebView

// Another function's parameter of the same name must not decide the property's type.
class Params(private val webView: Fake) {
    fun other(webView: WebView) {}

    fun go() {
        if (Build.VERSION.SDK_INT >= 26) {
            webView.getWebChromeClient()
        }
    }
}
