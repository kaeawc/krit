// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: a third-party WebView engine with the platform's class names and
// setter shape (for example Tencent X5, com.tencent.smtt.sdk). Enabling
// JavaScript on its WebSettings is the same XSS surface the message warns
// about. Go reports both calls through the simple name WebSettings, and so
// does FIR, which accepts a WebSettings class in any package.
package test.thirdparty

open class WebSettings {
    fun setJavaScriptEnabled(flag: Boolean) {
        println(flag)
    }
}

class WebView {
    val settings: WebSettings = WebSettings()
}

class TunedSettings : WebSettings()

fun configure(view: WebView, settings: WebSettings, tuned: TunedSettings) {
    <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>view.settings.setJavaScriptEnabled(true)<!>
    // Deliberate improvement: Go types the receiver as TunedSettings and does
    // not report it; FIR is correct because TunedSettings is a WebSettings.
    <!SetJavaScriptEnabled!>tuned.setJavaScriptEnabled(true)<!>
    settings.setJavaScriptEnabled(false)
}
