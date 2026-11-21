// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A lambda SAM-converted to javax.net.ssl.HostnameVerifier is the verifier's verify, so one that always returns true is
// reported on the lambda. Go only sees the `HostnameVerifier { ... }` constructor call and misses these.
package test.allowallhostnameverifiersamconversion

import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.HostnameVerifier
import okhttp3.OkHttpClient

fun installDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier <!BadHostnameVerifier!>{ _, _ -> true }<!>
}

fun okHttp(): OkHttpClient.Builder = OkHttpClient.Builder().hostnameVerifier <!BadHostnameVerifier!>{ _, _ -> true }<!>

fun parenthesized(connection: HttpsURLConnection) {
    connection.setHostnameVerifier(<!BadHostnameVerifier!>{ hostname, session -> true }<!>)
}

fun named(): OkHttpClient.Builder =
    OkHttpClient.Builder().hostnameVerifier(hostnameVerifier = <!BadHostnameVerifier!>{ _, _ -> true }<!>)

// A lambda that checks the host does not accept every host.
fun checksHost() {
    HttpsURLConnection.setDefaultHostnameVerifier { hostname, _ -> hostname == "example.com" }
}

// Go has no SAM finding here; FIR applies the same strict return-path rule
// as verifier members and ignores a false labeled return.
fun samFalseThenTrue() {
    val verifier = HostnameVerifier { host, _ ->
        if (host != "x") return@HostnameVerifier false
        true
    }
}

val samAllTrueBranches = <!BadHostnameVerifier!>HostnameVerifier<!> { host, _ ->
    if (host.isEmpty()) true else true
}

// A lambda that returns true for another functional interface is not a hostname verifier.
fun otherInterfaces(list: ArrayList<String>): java.util.function.BiPredicate<String, String> {
    list.removeIf { true }
    return java.util.function.BiPredicate { _, _ -> true }
}

fun strictDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ ->
        if (host != "x") return@setDefaultHostnameVerifier false
        true
    }
}

fun allTrueDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier <!BadHostnameVerifier!>{<!> host, _ ->
        if (host.isEmpty()) return@setDefaultHostnameVerifier true
        true
    }
}

// Every path returns the literal true, but the standard guards can reject a
// host by throwing; FIR stays silent. Go has no implicit-SAM coverage here.
fun guardedDefaults() {
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ -> require(host == "x"); true }
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ -> check(host == "x"); true }
}

// A throw path also rejects the host; Go has no implicit-SAM finding here.
fun throwingDefault() {
    HttpsURLConnection.setDefaultHostnameVerifier { host, _ ->
        if (host != "x") throw javax.net.ssl.SSLPeerUnverifiedException(host)
        true
    }
}
