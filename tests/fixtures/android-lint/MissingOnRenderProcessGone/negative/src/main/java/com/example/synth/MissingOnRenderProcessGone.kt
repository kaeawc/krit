package com.example.synth

import android.webkit.RenderProcessGoneDetail
import android.webkit.WebView
import android.webkit.WebViewClient

class BrowserClient : WebViewClient() {
    override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean = true
}

abstract class BaseClient : WebViewClient()

// A sealed class is abstract, which Lint skips.
sealed class SealedClient : WebViewClient()
