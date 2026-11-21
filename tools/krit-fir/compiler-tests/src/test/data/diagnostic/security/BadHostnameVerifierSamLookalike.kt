// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The named argument selects the factory's plain function parameter, not a
// verifier SAM. The positional call resolves to the JDK SAM constructor in FIR;
// Go conservatively rejects both because the import name is shadowed file-wide.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

fun HostnameVerifier(predicate: (String, SSLSession) -> Boolean): HostnameVerifier =
    javax.net.ssl.HostnameVerifier { host, _ -> host == "example.com" }

val factoryResult = HostnameVerifier(predicate = { _, _ -> true })
val actualSam = <!BadHostnameVerifier!>HostnameVerifier { _, _ -> true }<!>
