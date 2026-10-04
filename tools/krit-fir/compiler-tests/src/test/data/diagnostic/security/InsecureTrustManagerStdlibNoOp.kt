// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 18, 23, 24, 32, 37, 38, 45, 46, 53, 54
// Positive: check methods whose whole body is a stdlib call that only runs an
// empty lambda over the chain, or control flow with empty branches. Go reports
// these expression bodies because its brace scan finds the empty block, and
// the body validates nothing, so the message is true of the code. A null
// check (`!!`, `requireNotNull`, `require(chain != null)`) rejects no real
// certificate chain, so it does not count as validation either.
package test

import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class ForEachBody : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) = chain.forEach {}
    // An elvis whose left side does nothing and whose right side is `Unit`:
    // the form of a safe-call body that compiles as an expression body.
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = chain?.let {} ?: Unit
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class LockBody : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = synchronized(this) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = repeat(chain?.size ?: 0) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class IndexedBody : X509TrustManager {
    // Go misses this one: it reads the lambda's `_, _ ->` as body text. The
    // lambda still does nothing.
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = chain.orEmpty().forEachIndexed { _, _ -> }
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = chain!!.forEach {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class NullCheckBody : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = requireNotNull(chain).let {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = require(chain != null) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// Control flow whose branches do nothing: Go's brace scan finds the first
// empty branch.
class ControlFlowBody : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = if (chain == null) {} else {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) = try {} catch (e: Exception) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// Overloads with other parameters count too (neither Go nor FIR inspects the
// parameters); these bodies return a value nobody reads.
class ListOverloads : X509TrustManager {
    <!InsecureTrustManager!>fun<!> checkClientTrusted(chain: List<X509Certificate>) = chain.forEach {}
    <!InsecureTrustManager!>fun<!> checkServerTrusted(chain: List<X509Certificate>) = chain.onEach {}.map {}
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = checkClientTrusted(chain.orEmpty().asList())
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        with(chain) { this?.asSequence()?.forEach {} }
        checkServerTrusted(chain.orEmpty().toList())
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
