// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A local HostnameVerifier lookalike: its verify accepting everything is not
// a TLS hostname check, so neither Go nor FIR reports it. This comment
// mentions javax.net.ssl.HostnameVerifier, which is enough for Go's file gate.
package test

import javax.net.ssl.SSLSession

interface HostnameVerifier {
    fun verify(hostname: String, session: SSLSession): Boolean
}

class LocalVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

// Deliberate improvement: the qualified JDK interface is still a verifier. Go
// misses it because the file declares its own HostnameVerifier, which makes
// Go skip every class in the file.
class JdkVerifier : javax.net.ssl.HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
