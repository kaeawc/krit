// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12, 15, 16, 17
// Apache constructions and field references, not custom verify implementations.
package test

import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory

val constructed = <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier<!>()
val qualified = <!AllowAllHostnameVerifier!>org.apache.http.conn.ssl.AllowAllHostnameVerifier<!>()
val constant = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
val qualifiedConstant = <!AllowAllHostnameVerifier!>org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>

val custom = javax.net.ssl.HostnameVerifier { _, _ -> true }
val constructorReference = <!AllowAllHostnameVerifier!>::AllowAllHostnameVerifier<!>
val fieldReference = <!AllowAllHostnameVerifier!>SSLSocketFactory::ALLOW_ALL_HOSTNAME_VERIFIER<!>
val description = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>.toString()
