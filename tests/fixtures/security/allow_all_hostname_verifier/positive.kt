package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AllowAllExpression : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllBlock : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return true
    }
}
