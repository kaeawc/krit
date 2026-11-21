// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go conservatively treats any same-file declaration of an imported name as
// shadowing it everywhere. FIR resolves the unshadowed use in another scope.
package test

import org.apache.http.conn.ssl.SSLSocketFactory

fun lookalike(SSLSocketFactory: OtherFactory) = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER

class OtherFactory {
    val ALLOW_ALL_HOSTNAME_VERIFIER = Any()
}

val real = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
