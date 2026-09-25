// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: JavaScript left disabled, a value that is not the literal `true`
// (Go reads only a boolean literal), other WebSettings setters, and an
// unrelated property reached through `settings`. None of these report
// SetJavaScriptEnabled.
package test

import android.webkit.WebSettings
import android.webkit.WebView

const val ENABLE_SCRIPTS = true

class ScriptOptions {
    var javaScriptEnabled: Boolean = false
}

val WebSettings.extra: ScriptOptions
    get() = ScriptOptions()

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

    fun otherSetters(settings: WebSettings) {
        settings.domStorageEnabled = true
        settings.setAllowFileAccess(true)
        settings.allowContentAccess = true
    }

    // Precision fix: Go's WebView-parameter fallback accepts any chain through
    // `settings`, so it reports this; FIR is correct because the property
    // belongs to ScriptOptions, not WebSettings.
    fun unrelatedPropertyThroughSettings(view: WebView) {
        view.settings.extra.javaScriptEnabled = true
    }

    fun readsOnly(settings: WebSettings): Boolean {
        val enabled = settings.javaScriptEnabled
        return enabled && webView.settings.getJavaScriptEnabled()
    }
}
