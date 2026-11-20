// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Both engines stop at initializer, constructor, accessor and nested owner
// boundaries: these helpers are not the outer verifier's implementation.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Strict() : HostnameVerifier {
    override fun verify(host: String, session: SSLSession) = host == session.peerHost

    init {
        fun verify(host: String, session: SSLSession) = true
    }

    constructor(value: Int) : this() {
        fun verify(host: String, session: SSLSession) = true
    }

    var probe: Boolean = false
        get() {
            fun verify(host: String, session: SSLSession) = true
            return false
        }
        set(value) {
            fun verify(host: String, session: SSLSession) = true
        }

    class Helper { fun verify(host: String, session: SSLSession) = true }
    interface Fallback { fun verify(host: String, session: SSLSession) = true }
    object Local { fun verify(host: String, session: SSLSession) = true }
}
