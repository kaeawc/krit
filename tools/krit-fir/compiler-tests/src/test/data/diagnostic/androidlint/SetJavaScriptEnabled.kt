// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: enabling JavaScript on android.webkit.WebSettings (a Java stub),
// through the setter or the synthetic javaScriptEnabled property, with any
// receiver shape. Findings sit on the first line of the call or assignment.
package test

import android.webkit.WebSettings
import android.webkit.WebView

class BrowserHost(private val webView: WebView, private val stored: WebSettings) {
    fun setterOnSettingsChain(view: WebView) {
        <!SetJavaScriptEnabled!>view.settings.setJavaScriptEnabled(true)<!>
    }

    fun setterThroughGetter(view: WebView) {
        <!SetJavaScriptEnabled!>view.getSettings().setJavaScriptEnabled(true)<!>
    }

    fun setterOnParameter(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>
    }

    fun setterOnLocal(view: WebView) {
        val settings = view.settings
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>
    }

    fun setterOnProperty() {
        <!SetJavaScriptEnabled!>stored.setJavaScriptEnabled(true)<!>
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

    fun implicitReceivers(view: WebView) {
        view.settings.apply { <!SetJavaScriptEnabled!>javaScriptEnabled = true<!> }
        with(view.settings) { <!SetJavaScriptEnabled!>setJavaScriptEnabled(true)<!> }
        view.settings.run {
            <!SetJavaScriptEnabled!>javaScriptEnabled = true<!>
        }
    }

    fun insideLambda(views: List<WebView>) {
        views.forEach { <!SetJavaScriptEnabled!>it.settings.setJavaScriptEnabled(true)<!> }
    }

    fun twoOnOneLine(settings: WebSettings) {
        <!SetJavaScriptEnabled!>settings.setJavaScriptEnabled(true)<!>; settings.javaScriptEnabled = true
    }
}

abstract class CustomSettings : WebSettings()

fun enableCustom(custom: CustomSettings) {
    <!SetJavaScriptEnabled!>custom.setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>custom.javaScriptEnabled = true<!>
}

fun WebSettings.enableScripts() {
    <!SetJavaScriptEnabled!>javaScriptEnabled = true<!>
    <!SetJavaScriptEnabled!>setJavaScriptEnabled(true)<!>
    <!SetJavaScriptEnabled!>this.javaScriptEnabled = true<!>
}

val topLevelLambda: (WebView) -> Unit = { <!SetJavaScriptEnabled!>it.settings.javaScriptEnabled = true<!> }
