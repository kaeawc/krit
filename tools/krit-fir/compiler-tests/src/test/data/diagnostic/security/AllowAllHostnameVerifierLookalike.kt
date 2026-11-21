// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class AllowAllHostnameVerifier
object SSLSocketFactory {
    val ALLOW_ALL_HOSTNAME_VERIFIER = Any()
}

val constructed = AllowAllHostnameVerifier()
val constant = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
val text = "org.apache.http.conn.ssl.AllowAllHostnameVerifier()"
/*
import org.apache.http.conn.ssl.AllowAllHostnameVerifier
*/

fun local(SSLSocketFactory: Any) = SSLSocketFactory.toString()
