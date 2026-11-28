package com.example

import android.os.Build
import android.webkit.WebView

// This file's with() runs the block without a receiver: an unqualified call in it is the enclosing class's.
fun with(view: WebView, block: () -> Unit) = block()

class Own {
    fun getWebChromeClient(): Any? = null

    fun call(webView: WebView) {
        if (Build.VERSION.SDK_INT >= 26) {
            with(webView) { getWebChromeClient() }
        }
    }
}
