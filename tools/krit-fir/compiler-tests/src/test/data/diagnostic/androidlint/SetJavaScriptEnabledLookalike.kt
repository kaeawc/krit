// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: local lookalikes named WebSettings/WebView with the same members.
// Only the android.webkit owner counts; Go's simple-name fallback would fire
// here, and not firing is a deliberate precision fix.
package test.lookalike

class WebSettings {
    var javaScriptEnabled: Boolean = false
}

class WebView {
    val settings: WebSettings = WebSettings()
}

class ScriptToggle {
    fun setJavaScriptEnabled(flag: Boolean) {
        println(flag)
    }
}

fun setJavaScriptEnabled(flag: Boolean) {
    println(flag)
}

fun configure(view: WebView, settings: WebSettings, toggle: ScriptToggle) {
    view.settings.javaScriptEnabled = true
    settings.javaScriptEnabled = true
    toggle.setJavaScriptEnabled(true)
    setJavaScriptEnabled(true)
}

// Members of anonymous objects: their owner is a local class, which must not
// be looked up by class id (that lookup throws and aborts the compilation's
// checkers).
fun anonymousLookalikes() {
    val toggle = object {
        fun setJavaScriptEnabled(flag: Boolean) = println(flag)
    }
    toggle.setJavaScriptEnabled(true)
    toggle.setJavaScriptEnabled(false)
    val options = object {
        var javaScriptEnabled = false
    }
    options.javaScriptEnabled = true
    options.javaScriptEnabled = false
}
