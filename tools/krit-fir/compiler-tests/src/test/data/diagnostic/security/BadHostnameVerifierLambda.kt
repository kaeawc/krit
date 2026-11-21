// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12
// Positive: a SAM-constructor lambda that always returns true is reported on
// the call, as Go reports it.
package test

import javax.net.ssl.HostnameVerifier

val trustAll = <!BadHostnameVerifier!>HostnameVerifier { _, _ -> true }<!>
val parenthesized = <!BadHostnameVerifier!>HostnameVerifier { _, _ -> (true) }<!>
val explicitReturn = <!BadHostnameVerifier!>HostnameVerifier { _, _ -> return@HostnameVerifier true }<!>
val argument = <!BadHostnameVerifier!>HostnameVerifier({ _, _ -> true })<!>

val strict = HostnameVerifier { host, _ ->
    if (host != "example.com") return@HostnameVerifier false
    true
}

val nonliteral = HostnameVerifier { host, _ ->
    if (host.isEmpty()) return@HostnameVerifier host == "example.com"
    true
}

val allTrue = <!BadHostnameVerifier!>HostnameVerifier<!> { host, _ ->
    if (host.isEmpty()) return@HostnameVerifier true
    true
}

val nestedReturn = <!BadHostnameVerifier!>HostnameVerifier<!> { _, _ ->
    fun unrelated(): Boolean = false
    true
}
