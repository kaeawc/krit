package com.example

import android.webkit.JavascriptInterface

class Bridge {
    @JavascriptInterface
    private fun hidden() = Unit

    @get:JavascriptInterface
    protected val secret: String = "x"
}

open class CtorBridge(@get:JavascriptInterface protected val token: String)
