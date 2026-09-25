// RENDER_DIAGNOSTICS_FULL_TEXT
// Lookalikes of WebSettings/WebView. A class named WebSettings in any package
// is web settings to Go (it matches the simple name), and
// `javaScriptEnabled = true` on one is the finding the message describes, so
// FIR reports those two assignments as Go does. Setters on classes with other
// names, a top-level function, and members of anonymous objects are not
// WebSettings members and report in neither Go nor FIR.
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
    <!SetJavaScriptEnabled!>view.settings.javaScriptEnabled = true<!>
    <!SetJavaScriptEnabled!>settings.javaScriptEnabled = true<!>
    settings.javaScriptEnabled = false
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
