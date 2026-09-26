// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses: trust managers whose check method does nothing,
// which Go's text matching does not see as trust managers or as empty bodies.
package test

import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import java.net.Socket
import javax.net.ssl.SSLEngine
import javax.net.ssl.X509ExtendedTrustManager
import javax.net.ssl.X509TrustManager
import javax.net.ssl.X509TrustManager as Tm

typealias CertTrust = X509TrustManager

// Go does not visit object declarations.
object TrustEveryone : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// Go skips any class whose text contains " by ": this one delegates, but
// overrides the server check with an empty body, so that check never reaches
// the delegate.
class PartlyDelegated(delegate: X509TrustManager) : X509TrustManager by delegate {
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

// Go skips this class too, for its `by lazy` property.
class LazyIssuers : X509TrustManager {
    private val issuers by lazy { emptyArray<X509Certificate>() }
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = issuers
}

// Go needs the word TrustManager or X509TrustManager in the class text; an
// import alias, a type alias, and X509ExtendedTrustManager do not have it.
class Aliased : Tm {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class TypeAliased : CertTrust {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no servers")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class Extended : X509ExtendedTrustManager() {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?, socket: Socket?) {}
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?, engine: SSLEngine?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?, socket: Socket?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?, engine: SSLEngine?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// A subclass of a project base class: the class text never names a trust
// manager.
abstract class StrictBase : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no clients")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class LaxServer : StrictBase() {
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}

// Go only accepts an empty body or a bare `return`; these bodies do nothing
// either.
class UnitBodies : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) = Unit
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        Unit
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class ReturnUnit : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        return Unit
    }
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        run {}
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

class SafeCallBody : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        chain?.let {}
    }
    // A computed receiver may validate: not a do-nothing body.
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        requireNotNull(chain).also {}
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
