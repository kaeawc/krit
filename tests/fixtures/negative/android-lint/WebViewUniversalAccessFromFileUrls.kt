package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowUniversalAccessFromFileURLs = false
        webView.settings.setAllowUniversalAccessFromFileURLs(false)
    }
}

// String literal lookalike — must not trigger.
class StringLiteralOnly {
    fun describe(): String =
        "webView.settings.setAllowUniversalAccessFromFileURLs(true)"
}

// Non-WebView class with a property of the same name — must not trigger.
class FakeSettings {
    var allowUniversalAccessFromFileURLs: Boolean = false
}

fun nonWebViewCall() {
    val s = FakeSettings()
    s.allowUniversalAccessFromFileURLs = true
}
