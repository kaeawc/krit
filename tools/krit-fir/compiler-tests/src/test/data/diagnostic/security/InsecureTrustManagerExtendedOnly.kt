// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A true positive Go misses: Go requires the file to import or mention
// javax.net.ssl.TrustManager or javax.net.ssl.X509TrustManager, and this file
// only names X509ExtendedTrustManager, a subclass of both.
package test

import java.security.cert.X509Certificate
import java.net.Socket
import javax.net.ssl.SSLEngine
import javax.net.ssl.X509ExtendedTrustManager

val insecure = object : X509ExtendedTrustManager() {
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?, socket: Socket?) {}
    <!InsecureTrustManager!>override<!> fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?, engine: SSLEngine?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?, socket: Socket?) {}
    <!InsecureTrustManager!>override<!> fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?, engine: SSLEngine?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
