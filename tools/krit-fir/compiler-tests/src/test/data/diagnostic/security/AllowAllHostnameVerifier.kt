// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 13, 16, 17, 18
// Apache constructions and field references, not custom verify implementations.
package test

import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory
import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER

val constructed = <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier<!>()
val qualified = <!AllowAllHostnameVerifier!>org.apache.http.conn.ssl.AllowAllHostnameVerifier<!>()
val constant = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
val qualifiedConstant = <!AllowAllHostnameVerifier!>org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>

val custom = javax.net.ssl.HostnameVerifier { _, _ -> true }
val constructorReference = <!AllowAllHostnameVerifier!>::AllowAllHostnameVerifier<!>
val fieldReference = <!AllowAllHostnameVerifier!>SSLSocketFactory::ALLOW_ALL_HOSTNAME_VERIFIER<!>
val description = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>.toString()

// FIR resolves this callable reference and preserves the Kotlin finding; Go
// misses the referenced field because the tree-sitter parent-child shape is
// excluded by the unchanged Go visitor.
val callableReference = <!AllowAllHostnameVerifier!>::ALLOW_ALL_HOSTNAME_VERIFIER<!>
