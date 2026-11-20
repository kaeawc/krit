// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Named-argument labels are silent; the argument value remains a field use.
package test

import okhttp3.OkHttpClient
import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as hostnameVerifier

val strict = javax.net.ssl.HostnameVerifier { host, _ -> host == "example.com" }
val safe = OkHttpClient.Builder().hostnameVerifier(hostnameVerifier = strict)
val unsafe = OkHttpClient.Builder().hostnameVerifier(
    hostnameVerifier = <!AllowAllHostnameVerifier!>hostnameVerifier<!>,
)
