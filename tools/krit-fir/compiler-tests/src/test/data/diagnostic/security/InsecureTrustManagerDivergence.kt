// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 34, 40, 48, 56, 69, 91, 92
// Go findings FIR drops. Go takes any function named checkServerTrusted or
// checkClientTrusted whose nearest enclosing class declaration or object
// expression mentions the word TrustManager anywhere in its text, and treats
// the first brace block after the name as the body. None of these is a trust
// manager check method that accepts certificates without validation.
package test

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class WithLocalCheck : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        // Go reports this because the enclosing class is a trust manager; FIR
        // is correct to drop it because a local function is not its check
        // method.
        fun checkServerTrusted() {}
        checkServerTrusted()
        throw CertificateException("no clients")
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

    companion object {
        // Go reports this because tree-sitter skips the companion and finds the
        // trust manager class; FIR is correct to drop it because the companion
        // is not a trust manager.
        fun checkServerTrusted(host: String) {}
    }

    object Nested {
        // Go reports this for the same reason: a nested object that is not a
        // trust manager.
        fun checkClientTrusted(host: String) {}
    }
}

class TrustHolder(private val manager: X509TrustManager) {
    // Go reports this because the class text mentions X509TrustManager in a
    // property type; FIR is correct to drop it because the class is not a
    // trust manager.
    fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

class ExpressionBodies(private val verify: (Array<X509Certificate>?, () -> Unit) -> Unit) : X509TrustManager {
    // Go reports this because the first brace block after the name is the
    // empty lambda. FIR drops it as delegation, like the Delegating negative
    // Go also leaves alone: the body hands the chain to a callee that is not a
    // do-nothing stdlib function, so nothing shows it accepts every chain.
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = verify(chain) {}

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class DefaultLambda : X509TrustManager {
    // Go reports this because the first brace block after the name is the
    // default argument `{}`; FIR is correct to drop it because the body
    // throws on an empty chain.
    fun checkServerTrusted(chain: List<X509Certificate>, onFail: () -> Unit = {}) {
        if (chain.isEmpty()) throw CertificateException("empty")
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no clients")
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        checkServerTrusted(chain.orEmpty().asList())
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

interface TrustProvider<T>

// The shape of tests/fixtures/negative/android-lint/TrustedServer.kt. Go
// reports both because the class text names X509TrustManager as a type
// argument; FIR is correct to drop them because the class is not a trust
// manager.
class TypeArgumentOnly : TrustProvider<X509TrustManager> {
    fun checkClientTrusted() {}
    fun checkServerTrusted() {}
}
