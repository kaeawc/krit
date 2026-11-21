package stubs

import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
import java.security.KeyStore

fun apacheVerifier(verifier: AllowAllHostnameVerifier, session: SSLSession): Boolean {
    val jdkVerifier: HostnameVerifier = verifier
    return jdkVerifier.verify("example.com", session)
}

fun apacheSocketFactory(keyStore: KeyStore): SSLSocketFactory = SSLSocketFactory(keyStore)
