// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A lambda SAM-converted to javax.net.ssl.HostnameVerifier is the verifier's verify, so one that always returns true is
// reported on the lambda. Go only sees the `HostnameVerifier { ... }` constructor call and misses these.
package test.allowallhostnameverifiersamconversion

import javax.net.ssl.HttpsURLConnection
import okhttp3.OkHttpClient

fun installDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier <!AllowAllHostnameVerifier!>{ _, _ -> true }<!>
}

fun okHttp(): OkHttpClient.Builder = OkHttpClient.Builder().hostnameVerifier <!AllowAllHostnameVerifier!>{ _, _ -> true }<!>

fun parenthesized(connection: HttpsURLConnection) {
    connection.setHostnameVerifier(<!AllowAllHostnameVerifier!>{ hostname, session -> true }<!>)
}

fun named(): OkHttpClient.Builder =
    OkHttpClient.Builder().hostnameVerifier(hostnameVerifier = <!AllowAllHostnameVerifier!>{ _, _ -> true }<!>)

// A lambda that checks the host does not accept every host.
fun checksHost() {
    HttpsURLConnection.setDefaultHostnameVerifier { hostname, _ -> hostname == "example.com" }
}

// A lambda that returns true for another functional interface is not a hostname verifier.
fun otherInterfaces(list: ArrayList<String>): java.util.function.BiPredicate<String, String> {
    list.removeIf { true }
    return java.util.function.BiPredicate { _, _ -> true }
}
