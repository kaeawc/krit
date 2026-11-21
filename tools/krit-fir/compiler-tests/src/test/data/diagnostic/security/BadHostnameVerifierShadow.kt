// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
// An unrelated parameter does not shadow this class's JDK interface import.
// Both engines still report the real verifier member.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

fun helper(HostnameVerifier: Any) = HostnameVerifier.toString()

class Real : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}
