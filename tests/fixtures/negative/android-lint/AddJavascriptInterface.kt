package com.example

import android.webkit.WebView

class MyWebView {
    fun setup(webView: WebView) {
        webView.loadUrl("https://example.com")
    }
}

// Comment: do not call addJavascriptInterface(
class CommentOnly {
    fun explain(webView: WebView) {
        // Historically we used to call webView.addJavascriptInterface(bridge, "Android")
        webView.loadUrl("about:blank")
    }
}

class StringLiteralOnly {
    fun describe(): String {
        return "addJavascriptInterface(bridge, \"Android\")"
    }
}

class NonWebViewWrapper {
    @Suppress("EmptyFunctionBlock")
    fun addJavascriptInterface(obj: Any, name: String) {}
}

fun nonWebViewCall() {
    val wrapper = NonWebViewWrapper()
    wrapper.addJavascriptInterface(Any(), "bridge")
}
