// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves wildcard imports; Go intentionally requires an explicit import or FQN.
package test

import org.apache.http.conn.ssl.*

val constructed = <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier<!>()
val constant = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
