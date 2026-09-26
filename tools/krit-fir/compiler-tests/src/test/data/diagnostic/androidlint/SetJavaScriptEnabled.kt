// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 25, 36, 43, 49, 53, 59, 63, 77, 82, 86, 139x2, 224
// Positive: enabling JavaScript on android.webkit.WebSettings (a Java stub),
// through the setter or the synthetic javaScriptEnabled property, with any
// receiver shape. Findings sit on the first line of the call or assignment.
// Go reports only some of these (the go-lines header); each group it misses
// is marked below.
package test

import android.webkit.WebSettings
import android.webkit.WebView

class BrowserHost(private val webView: WebView, private val stored: WebSettings) {
    fun setterOnSettingsChain(view: WebView) {
        <!SetJavaScriptEnabled!>view.settings.setJavaScriptEnabled(true)<!>
    }

    // Deliberate improvement: Go misses this. It cannot type view.getSettings(),
    // and its WebView-parameter fallback needs a `settings` segment.
    fun setterThroughGetter(view: WebView) {
        <!SetJavaScriptEnabled!>view.getSettings().setJavaScriptEnabled(true)<!>
    }

    fun setterOnParameter(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>
    }

    // Deliberate improvement: Go misses this; it cannot type the local
    // `settings`, which is inferred from view.settings.
    fun setterOnLocal(view: WebView) {
        val settings = view.settings
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>
    }

    fun setterOnProperty() {
        <!SetJavaScriptEnabled!>stored.setJavaScriptEnabled(true)<!>
        // Go misses this one: its WebView fallback needs a WebView parameter,
        // and webView is a property.
        <!SetJavaScriptEnabled!>webView.settings.setJavaScriptEnabled(true)<!>
    }

    fun multiLineSetter(view: WebView) {
        <!SetJavaScriptEnabled!>view<!>
            .settings
            .setJavaScriptEnabled(true)
    }

    fun safeCallSetter(view: WebView?) {
        <!SetJavaScriptEnabled!>view?.settings?.setJavaScriptEnabled(true)<!>
    }

    fun multiLineSafeCallSetter(view: WebView?) {
        <!SetJavaScriptEnabled!>view<!>
            ?.settings
            ?.setJavaScriptEnabled(true)
    }

    fun propertyOnSettingsChain(view: WebView) {
        <!SetJavaScriptEnabled!>view.settings.javaScriptEnabled = true<!>
    }

    fun propertyOnParameter(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.javaScriptEnabled = true<!>
    }

    // Deliberate improvement: Go misses all three. It reports a
    // javaScriptEnabled assignment only when the receiver is a WebSettings
    // parameter or a `settings` chain on a WebView parameter.
    fun propertyOnLocalAndProperty(view: WebView) {
        val settings = view.settings
        <!SetJavaScriptEnabled!>settings.javaScriptEnabled = true<!>
        <!SetJavaScriptEnabled!>stored.javaScriptEnabled = true<!>
        <!SetJavaScriptEnabled!>this.webView.settings.javaScriptEnabled = true<!>
    }

    fun propertySplitAcrossLines(view: WebView) {
        <!SetJavaScriptEnabled!>view.settings.javaScriptEnabled =<!>
            true
    }

    fun safePropertyAssignment(view: WebView?) {
        <!SetJavaScriptEnabled!>view?.settings?.javaScriptEnabled = true<!>
    }

    fun multiLineSafePropertyAssignment(view: WebView?) {
        <!SetJavaScriptEnabled!>view<!>
            ?.settings
            ?.javaScriptEnabled = true
    }

    // Deliberate improvement: Go sees only an identifier or navigation receiver
    // and does not type these; FIR resolves every one to WebSettings.
    fun wrappedReceivers(view: WebView?, settings: WebSettings?, fallback: WebSettings) {
        <!SetJavaScriptEnabled!>settings!!.setJavaScriptEnabled(true)<!>
        <!SetJavaScriptEnabled!>settings!!.javaScriptEnabled = true<!>
        <!SetJavaScriptEnabled!>(fallback).setJavaScriptEnabled(true)<!>
        <!SetJavaScriptEnabled!>(view!!.settings).javaScriptEnabled = true<!>
        <!SetJavaScriptEnabled!>(view?.settings ?: fallback).javaScriptEnabled = true<!>
        <!SetJavaScriptEnabled!>(view?.settings ?: fallback).setJavaScriptEnabled(true)<!>
    }

    // Deliberate improvement: Go requires the argument or right-hand side to be a
    // bare boolean_literal node, so it misses a parenthesized, annotated, or
    // labeled `true`; FIR is correct because each is still the literal `true`.
    fun wrappedTrueLiteral(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled((true))<!>
        <!SetJavaScriptEnabled!>settings.javaScriptEnabled = (true)<!>
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(@Suppress("UNUSED") true)<!>
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(flag@ true)<!>
    }

    // Deliberate improvement: the property initializer and the init block still
    // enable JavaScript. Go misses both, as it misses the same receivers inside
    // a function above: a `settings` chain on a WebView property, and a
    // javaScriptEnabled assignment whose receiver is not a parameter.
    val enabledAtInit = <!SetJavaScriptEnabled!>webView.settings.setJavaScriptEnabled(true)<!>

    init {
        <!SetJavaScriptEnabled!>stored.javaScriptEnabled = true<!>
    }

    // Deliberate improvement: Go misses calls and assignments on the implicit
    // receiver of a scope function.
    fun implicitReceivers(view: WebView) {
        view.settings.apply { <!SetJavaScriptEnabled!>javaScriptEnabled = true<!> }
        with(view.settings) { <!SetJavaScriptEnabled!>setJavaScriptEnabled(true)<!> }
        view.settings.run {
            <!SetJavaScriptEnabled!>javaScriptEnabled = true<!>
        }
    }

    // Deliberate improvement: Go misses this; the receiver root is the lambda
    // parameter `it`, not a WebView parameter.
    fun insideLambda(views: List<WebView>) {
        views.forEach { <!SetJavaScriptEnabled!>it.settings.setJavaScriptEnabled(true)<!> }
    }

    fun twoOnOneLine(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>; settings.javaScriptEnabled = true
    }
}

// Deliberate improvement: Go misses these; the receiver's type is
// CustomSettings, a WebSettings subclass whose name Go does not match.
abstract class CustomSettings : WebSettings()

fun enableCustom(custom: CustomSettings) {
    <!SetJavaScriptEnabled!>custom.setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>custom.javaScriptEnabled = true<!>
}

// Deliberate improvement: Go misses these; an extension receiver, implicit or
// `this`, is not a parameter.
fun WebSettings.enableScripts() {
    <!SetJavaScriptEnabled!>javaScriptEnabled = true<!>
    <!SetJavaScriptEnabled!>setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>this.javaScriptEnabled = true<!>
}

// Deliberate improvement: Go misses this; the receiver root is the lambda
// parameter `it`, outside any function.
val topLevelLambda: (WebView) -> Unit = { <!SetJavaScriptEnabled!>it.settings.javaScriptEnabled = true<!> }

// A Kotlin override of the setter is still WebSettings.setJavaScriptEnabled.
abstract class NamedSettings : WebSettings() {
    override fun setJavaScriptEnabled(flag: Boolean) {}
}

// Deliberate improvement: Go reads only the first unlabeled argument and skips
// `flag = true`; FIR is correct because the named argument is still the value.
fun enableNamed(settings: NamedSettings) {
    <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(flag = true)<!>
    settings.setJavaScriptEnabled(flag = false)
}

// An anonymous WebSettings subclass is still a WebSettings, like CustomSettings
// above: the override is called with `true` from inside the object and through
// a local. Deliberate improvement: Go cannot type the anonymous receiver.
abstract class OpenSettings : WebSettings() {
    override fun getJavaScriptEnabled(): Boolean = false
    override fun getDomStorageEnabled(): Boolean = false
    override fun setDomStorageEnabled(flag: Boolean) {}
    override fun getAllowFileAccess(): Boolean = false
    override fun setAllowFileAccess(allow: Boolean) {}
    override fun getAllowContentAccess(): Boolean = false
    override fun setAllowContentAccess(allow: Boolean) {}
    @Deprecated("stub")
    override fun getAllowFileAccessFromFileURLs(): Boolean = false
    @Deprecated("stub")
    override fun setAllowFileAccessFromFileURLs(flag: Boolean) {}
    @Deprecated("stub")
    override fun getAllowUniversalAccessFromFileURLs(): Boolean = false
    @Deprecated("stub")
    override fun setAllowUniversalAccessFromFileURLs(flag: Boolean) {}
    override fun getMixedContentMode(): Int = 0
    override fun setMixedContentMode(mode: Int) {}
    override fun getUserAgentString(): String = ""
    override fun setUserAgentString(ua: String) {}
}

fun anonymousSettings(): WebSettings {
    val fake = object : OpenSettings() {
        override fun setJavaScriptEnabled(flag: Boolean) {}

        fun enable() {
            <!SetJavaScriptEnabled!>setJavaScriptEnabled(true)<!>
        }
    }
    <!SetJavaScriptEnabled!>fake.setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>fake.javaScriptEnabled = true<!>
    fake.enable()
    return fake
}

// A user overload on WebSettings with `true` as its first argument: Go reports
// it through the WebSettings receiver type, and so does FIR (a wrapper that
// takes the enable flag first still enables JavaScript).
fun WebSettings.setJavaScriptEnabled(enabled: Boolean, log: Boolean) {
    setJavaScriptEnabled(enabled)
    if (log) <!PrintlnInProduction!>println<!>(enabled)
}

fun enableThroughOverload(settings: WebSettings) {
    <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true, false)<!>
    settings.setJavaScriptEnabled(false, true)
}
