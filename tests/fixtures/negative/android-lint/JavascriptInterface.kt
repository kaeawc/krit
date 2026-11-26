package com.example

import android.webkit.JavascriptInterface

class Bridge {
    @JavascriptInterface
    fun visible() = Unit

    @get:JavascriptInterface
    val name: String = "x"

    // A private property with default accessors has no getter to annotate.
    @get:JavascriptInterface
    private val secret: String = "x"

    @field:JavascriptInterface
    private val field: String = "x"
}

class CtorBridge(@get:JavascriptInterface val token: String)
