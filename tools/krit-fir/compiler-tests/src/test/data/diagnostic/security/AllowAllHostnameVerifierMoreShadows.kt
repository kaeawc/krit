// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Both engines exclude constructor parameters, named objects and companions
// that shadow the imported Apache class.
package test

import org.apache.http.conn.ssl.SSLSocketFactory

class OtherFactory { val ALLOW_ALL_HOSTNAME_VERIFIER = false }
class Holder(val SSLSocketFactory: OtherFactory) {
    fun value() = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
}

class ObjectConfig {
    object SSLSocketFactory { const val ALLOW_ALL_HOSTNAME_VERIFIER = "x" }
    val value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
}

class CompanionConfig {
    companion object SSLSocketFactory { const val ALLOW_ALL_HOSTNAME_VERIFIER = "x" }
    val value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
}
