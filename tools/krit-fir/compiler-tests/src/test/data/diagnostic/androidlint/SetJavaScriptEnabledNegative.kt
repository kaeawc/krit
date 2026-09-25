// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: JavaScript left disabled, a value that is not a bare `true`
// literal (Go reads only a bare boolean literal), and other WebSettings
// setters. None of these report SetJavaScriptEnabled.
package test

import android.webkit.WebSettings
import android.webkit.WebView

const val ENABLE_SCRIPTS = true

class SafeBrowserHost(private val webView: WebView) {
    fun disabled(view: WebView, settings: WebSettings) {
        view.settings.setJavaScriptEnabled(false)
        view.settings.javaScriptEnabled = false
        settings.apply { javaScriptEnabled = false }
    }

    fun nonLiteralValues(settings: WebSettings, flag: Boolean) {
        settings.setJavaScriptEnabled(flag)
        settings.javaScriptEnabled = flag
        settings.setJavaScriptEnabled(ENABLE_SCRIPTS)
        settings.javaScriptEnabled = ENABLE_SCRIPTS
        settings.setJavaScriptEnabled(!false)
        settings.javaScriptEnabled = !false
        settings.setJavaScriptEnabled(flag || true)
    }

    fun parenthesizedTrue(settings: WebSettings) {
        settings.setJavaScriptEnabled((true))
        settings.javaScriptEnabled = (true)
    }

    fun otherSetters(settings: WebSettings) {
        settings.domStorageEnabled = true
        settings.setAllowFileAccess(true)
        settings.allowContentAccess = true
    }

    fun readsOnly(settings: WebSettings): Boolean {
        val enabled = settings.javaScriptEnabled
        return enabled && webView.settings.getJavaScriptEnabled()
    }
}
