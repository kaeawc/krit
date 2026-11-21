// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

object Strict : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        if (host != "x") return false
        return true
    }
}

class Holder {
    companion object : HostnameVerifier {
        override fun verify(host: String, session: SSLSession): Boolean {
            if (host != "x") return false
            return true
        }
    }
}

object AllTrue : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession): Boolean {
        if (host.isEmpty()) return true
        return true
    }
}

// A throwing path or a Kotlin guard can reject a hostname even though the
// sole explicit return is true, so strict FIR deliberately does not report.
object ThrowsOnMismatch : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        if (host != "x") throw javax.net.ssl.SSLPeerUnverifiedException(host)
        return true
    }
}

object RequiresMatch : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        require(host == "x")
        return true
    }
}

object ChecksMatch : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        check(host == "x")
        return true
    }
}
