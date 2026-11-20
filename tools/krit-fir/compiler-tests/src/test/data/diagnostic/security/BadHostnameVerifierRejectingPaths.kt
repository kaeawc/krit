// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 17, 24, 31, 38
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession

class Requires : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        require(host == "x")
        return true
    }
}

class RequiresNotNull : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        requireNotNull(host.takeIf { it == "x" })
        return true
    }
}

class Checks : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        check(host == "x")
        return true
    }
}

class ChecksNotNull : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        checkNotNull(host.takeIf { it == "x" })
        return true
    }
}

class Asserts : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession): Boolean {
        assert(host == "x")
        return true
    }
}

object RequiresNonNull : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        java.util.Objects.requireNonNull(host.takeIf { it == "x" })
        return true
    }
}

object ErrorsOnMismatch : HostnameVerifier {
    override fun verify(host: String, session: SSLSession): Boolean {
        if (host != "x") error("unexpected host $host")
        return true
    }
}

fun reject(host: String): Nothing = throw IllegalArgumentException(host)

val verifier = HostnameVerifier { host, _ -> if (host != "x") reject(host); true }

fun defaults() {
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ ->
        checkNotNull(host.takeIf { it == "x" })
        true
    }
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ ->
        requireNotNull(host.takeIf { it == "x" })
        true
    }
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ ->
        if (host != "x") TODO()
        true
    }
}
