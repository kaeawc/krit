package com.example

import android.os.Build
import android.webkit.WebView

class Fake {
    fun getWebChromeClient(): Any? = null
}

// Either value of a `when` entry can enter it: `25, 26` proves only API 25.
fun whenValues(webView: WebView) {
    when (Build.VERSION.SDK_INT) {
        25, 26 -> webView.getWebChromeClient()
        27, 28 -> webView.getWebViewClient()
    }
}

class Checked(private val webView: WebView) {
    fun ready(): Boolean = Build.VERSION.SDK_INT >= 26

    fun call() {
        if (ready()) {
            webView.getWebChromeClient()
        }
    }
}

// This class's own ready() proves nothing, whatever Checked.ready() checks.
class Unchecked(private val webView: WebView) {
    fun ready(): Boolean = true

    fun call() {
        if (ready()) {
            webView.getWebChromeClient()
        }
    }
}

// A local declared in the loop body hides the loop variable.
fun nearerLocal(views: List<WebView>, fake: Fake) {
    if (Build.VERSION.SDK_INT >= 26) {
        for (webView: WebView in views) {
            webView.getWebViewClient()
        }
        for (webView: WebView in views) {
            val webView: Fake = fake
            webView.getWebChromeClient()
        }
    }
}

fun staticSuffix() {
    if (Build.VERSION.SDK_INT >= 28) {
        WebView.setDataDirectorySuffix("x")
    }
}
