package stubs

import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory
import javax.net.ssl.HostnameVerifier

val apacheVerifier: HostnameVerifier = <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier<!>()
val apacheDefault: HostnameVerifier = <!AllowAllHostnameVerifier!>SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER<!>
