package test

import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory

val verifier = AllowAllHostnameVerifier()
val defaultVerifier = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
