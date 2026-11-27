package com.example

import android.content.Context
import android.os.Build
import android.webkit.WebChromeClient
import android.webkit.WebView

class MyWeb(context: Context) : WebView(context) {
    override fun getWebChromeClient(): WebChromeClient? = null

    fun postWebMessage(a: String, b: String) {}

    fun inside() {
        if (Build.VERSION.SDK_INT >= 26) {
            getWebChromeClient()
            postWebMessage("a", "b")
        }
    }
}

class Fake {
    fun getWebChromeClient(): Any? = null
}

class Review(private val webView: WebView) {
    fun subclass(w: MyWeb) {
        if (Build.VERSION.SDK_INT >= 26) {
            w.getWebChromeClient()
            w.postWebMessage("a", "b")
        }
    }

    // The loop variable hides the WebView property.
    fun loop(fakes: List<Fake>) {
        if (Build.VERSION.SDK_INT >= 26) {
            for (webView in fakes) {
                webView.getWebChromeClient()
            }
        }
    }

    fun inLambda() {
        if (Build.VERSION.SDK_INT >= 26) {
            run { webView.getWebChromeClient() }
        }
    }
}
