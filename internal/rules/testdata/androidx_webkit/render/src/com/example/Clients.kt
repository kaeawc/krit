package com.example

import android.webkit.RenderProcessGoneDetail
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.webkit.WebViewClientCompat

class NoHandler : WebViewClient()

class Handles : WebViewClient() {
    override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean = true
}

open class BaseHandles : WebViewClient() {
    override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean = true
}

class InheritsHandler : BaseHandles()

open class BaseNoHandler : WebViewClient()

class InheritsNoHandler : BaseNoHandler()

abstract class AbstractClient : WebViewClient()

class CompatNoHandler : WebViewClientCompat()

class WrongArity : WebViewClient() {
    fun onRenderProcessGone(view: WebView): Boolean = true
}

fun direct(view: WebView) {
    view.webViewClient = WebViewClient()
    view.webViewClient = object : WebViewClient() {}
    view.webViewClient = object : WebViewClient() {
        override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean = true
    }
    val compat = WebViewClientCompat()
    val mine = NoHandler()
}

class NotAClient

sealed class SealedClient : WebViewClient()

class WithInterface : WebViewClient(), Runnable {
    override fun run() = Unit
}
