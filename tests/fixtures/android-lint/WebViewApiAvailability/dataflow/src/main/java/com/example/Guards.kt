package com.example

import android.os.Build
import android.webkit.WebView
import androidx.annotation.ChecksSdkIntAtLeast

@ChecksSdkIntAtLeast(api = Build.VERSION_CODES.O)
fun annotatedAtLeastO(): Boolean = isO()

fun isO() = Build.VERSION.SDK_INT >= 26

class Fake {
    fun getWebChromeClient(): Any? = null
}

class Guards(private val webView: WebView) {
    fun booleanLocal() {
        val atLeastO = Build.VERSION.SDK_INT >= 26
        if (atLeastO) {
            webView.getWebChromeClient()
        }
    }

    fun annotatedHelper() {
        if (annotatedAtLeastO()) {
            webView.getWebChromeClient()
        }
    }

    fun oneLineHelper() {
        if (isO()) {
            webView.getWebChromeClient()
        }
    }

    fun whenRange() {
        when (Build.VERSION.SDK_INT) {
            in 26..40 -> webView.getWebChromeClient()
        }
    }

    fun whenElse() {
        when {
            Build.VERSION.SDK_INT < 26 -> println("old")
            else -> webView.getWebChromeClient()
        }
    }

    fun andRight(): Boolean = Build.VERSION.SDK_INT >= 26 && webView.getWebChromeClient() != null

    fun withReceiver() {
        if (Build.VERSION.SDK_INT >= 26) {
            with(webView) { getWebChromeClient() }
        }
    }

    fun anonymousObject() {
        if (Build.VERSION.SDK_INT >= 26) {
            val r = object : Runnable {
                override fun run() {
                    webView.getWebChromeClient()
                }
            }
            r.run()
        }
    }

    // A destructured lambda parameter hides the property.
    fun destructured(pairs: List<Pair<Fake, Int>>) {
        if (Build.VERSION.SDK_INT >= 26) {
            pairs.forEach { (webView, _) -> webView.getWebChromeClient() }
        }
    }
}

object Holder {
    // A nested class named WebView is not android.webkit.WebView.
    class WebView {
        fun getWebChromeClient(): Any? = null
    }

    fun nested(w: WebView) {
        if (Build.VERSION.SDK_INT >= 26) {
            w.getWebChromeClient()
        }
    }
}
