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
