// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 19, 20, 21, 23
// Values Lint's ConstantEvaluator folds to `true`, as Go's lintEvalBool reads them: a constant, `!false`, and a val
// whose initializer is one. Each enables JavaScript as the literal does.
package test.setjavascriptenabledconstant

import android.webkit.WebSettings
import android.webkit.WebView

const val ENABLE_SCRIPTS = true
private val scriptsOn = !false

object Config {
    const val SCRIPTS_ON = true
}

fun constants(view: WebView, settings: WebSettings) {
    <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(ENABLE_SCRIPTS)<!>
    <!SetJavaScriptEnabled!>settings.javaScriptEnabled = ENABLE_SCRIPTS<!>
    <!SetJavaScriptEnabled!>view.settings.javaScriptEnabled = !false<!>
    <!SetJavaScriptEnabled!>settings.javaScriptEnabled = scriptsOn<!>
    val local = ENABLE_SCRIPTS
    <!SetJavaScriptEnabled!>settings.javaScriptEnabled = local<!>
}

// Go follows only a bare name, so it misses the qualified constant.
fun qualified(settings: WebSettings) {
    <!SetJavaScriptEnabled!>settings.javaScriptEnabled = Config.SCRIPTS_ON<!>
}
