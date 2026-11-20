// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go conservatively treats the unrelated parameter name as shadowing the
// imported interface throughout the file. FIR resolves the class supertype.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

fun helper(HostnameVerifier: Any) = HostnameVerifier.toString()

class Real : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}
