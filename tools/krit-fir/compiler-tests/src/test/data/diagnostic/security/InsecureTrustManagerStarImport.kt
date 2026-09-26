// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 12
// A star import of javax.net.ssl: Go counts it as importing the trust manager
// types, and resolution sees the same class.
package test

import java.security.cert.X509Certificate
import javax.net.ssl.*

class StarTrustAll : X509TrustManager {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

// A lookalike interface in this package with a trust-manager-like name is not
// javax.net.ssl.TrustManager, and its text has no TrustManager word.
interface LocalTrustGate {
    fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?)
}

class OpenLocalGate : LocalTrustGate {
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
}
