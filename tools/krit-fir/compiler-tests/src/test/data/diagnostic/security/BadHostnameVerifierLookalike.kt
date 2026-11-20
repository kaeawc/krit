// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// Neither engine reports the local lookalike. Comments cannot establish type
// identity; the qualified JDK interface remains identifiable in both engines.
package test

import javax.net.ssl.SSLSession

interface HostnameVerifier {
    fun verify(hostname: String, session: SSLSession): Boolean
}

class LocalVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

// A local simple-name lookalike does not shadow the qualified JDK interface.
class JdkVerifier : javax.net.ssl.HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
