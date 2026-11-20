// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12
package test

import org.apache.http.conn.ssl.AllowAllHostnameVerifier as UnsafeVerifier
import org.apache.http.conn.ssl.SSLSocketFactory as ApacheFactory
import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as unsafe

val constructed = <!AllowAllHostnameVerifier!>UnsafeVerifier<!>()
val constant = <!AllowAllHostnameVerifier!>ApacheFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
val importedConstant = <!AllowAllHostnameVerifier!>unsafe<!>
val description = <!AllowAllHostnameVerifier!>unsafe<!>.toString()
