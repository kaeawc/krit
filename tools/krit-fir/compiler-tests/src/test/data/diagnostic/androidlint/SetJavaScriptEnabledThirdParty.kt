// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: a third-party WebView engine with the platform's class names and
// setter shape (for example Tencent X5, com.tencent.smtt.sdk). Go reports both
// calls because its fallbacks match the simple name WebSettings and a
// WebView-typed parameter chain through `settings`; FIR is correct because the
// rule, like Android Lint's, covers android.webkit.WebSettings only.
package test.thirdparty

class WebSettings {
    fun setJavaScriptEnabled(flag: Boolean) {
        println(flag)
    }
}

class WebView {
    val settings: WebSettings = WebSettings()
}

fun configure(view: WebView, settings: WebSettings) {
    settings.setJavaScriptEnabled(true)
    view.settings.setJavaScriptEnabled(true)
}
