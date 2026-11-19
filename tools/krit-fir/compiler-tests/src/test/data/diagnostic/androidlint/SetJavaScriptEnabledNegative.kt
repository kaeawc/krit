// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 50
// Negative: JavaScript left disabled, a value that is not known to be `true`
// (a parameter, a var, a call), other WebSettings setters, and an unrelated
// property reached through `settings`. None of these report
// SetJavaScriptEnabled.
package test

import android.webkit.WebSettings
import android.webkit.WebView

const val DISABLE_SCRIPTS = false
var mutableScripts = true

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
        settings.setJavaScriptEnabled(DISABLE_SCRIPTS)
        settings.javaScriptEnabled = !true
        settings.javaScriptEnabled = mutableScripts
        settings.javaScriptEnabled = flag.not()
        settings.setJavaScriptEnabled(flag || true)
    }

    fun otherSetters(settings: WebSettings) {
        settings.domStorageEnabled = true
        settings.setAllowFileAccess(true)
        settings.allowContentAccess = true
    }

    // Precision fix: Go's WebView-parameter fallback accepts any chain through
    // `settings`, so it reports this; FIR is correct because the property
    // belongs to ScriptOptions, not a WebSettings, so this code does not
    // enable JavaScript in a WebView as the message asserts.
    fun unrelatedPropertyThroughSettings(view: WebView) {
        view.settings.extra.javaScriptEnabled = true
    }

    fun readsOnly(settings: WebSettings): Boolean {
        val enabled = settings.javaScriptEnabled
        return enabled && webView.settings.getJavaScriptEnabled()
    }
}
