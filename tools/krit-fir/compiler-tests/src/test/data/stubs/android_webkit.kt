// Compiler-test source stubs; never packaged in the production artifact.
package android.webkit

import android.content.Context
import android.net.Uri
import android.net.http.SslError
import android.util.AttributeSet
import android.widget.AbsoluteLayout

// Java abstract class app code only reads from WebView.getSettings(); modeled
// as an object so the MIXED_CONTENT_* statics keep their Java callable ids.
object WebSettings {
    const val MIXED_CONTENT_ALWAYS_ALLOW: Int = 0
    const val MIXED_CONTENT_NEVER_ALLOW: Int = 1
    const val MIXED_CONTENT_COMPATIBILITY_MODE: Int = 2

    var javaScriptEnabled: Boolean
        get() = TODO()
        set(value) = TODO()

    var domStorageEnabled: Boolean
        get() = TODO()
        set(value) = TODO()

    var allowFileAccess: Boolean
        get() = TODO()
        set(value) = TODO()

    var allowContentAccess: Boolean
        get() = TODO()
        set(value) = TODO()

    @Deprecated("Deprecated in Java")
    var allowFileAccessFromFileURLs: Boolean
        get() = TODO()
        set(value) = TODO()

    @Deprecated("Deprecated in Java")
    var allowUniversalAccessFromFileURLs: Boolean
        get() = TODO()
        set(value) = TODO()

    var mixedContentMode: Int
        get() = TODO()
        set(value) = TODO()

    var userAgentString: String?
        get() = TODO()
        set(value) = TODO()
}

open class WebView : AbsoluteLayout {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    open val settings: WebSettings
        get() = TODO()

    open var webViewClient: WebViewClient
        get() = TODO()
        set(value) = TODO()

    open fun loadUrl(url: String) {
        TODO()
    }

    open fun loadUrl(url: String, additionalHttpHeaders: Map<String, String>) {
        TODO()
    }

    open fun addJavascriptInterface(obj: Any, name: String) {
        TODO()
    }

    open fun evaluateJavascript(script: String, resultCallback: ValueCallback<String>?) {
        TODO()
    }

    open fun destroy() {
        TODO()
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }
}

open class WebViewClient {
    open fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean = TODO()

    open fun onPageFinished(view: WebView, url: String?) {
        TODO()
    }

    open fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
        TODO()
    }
}

open class SslErrorHandler {
    open fun proceed() {
        TODO()
    }

    open fun cancel() {
        TODO()
    }
}

interface WebResourceRequest {
    val url: Uri

    val method: String

    val isForMainFrame: Boolean
}

fun interface ValueCallback<T> {
    fun onReceiveValue(value: T)
}

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class JavascriptInterface
