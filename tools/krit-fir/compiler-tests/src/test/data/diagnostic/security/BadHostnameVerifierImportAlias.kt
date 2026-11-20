// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// Explicit import aliases identify the JDK verifier in both Go and FIR.
package test

import javax.net.ssl.HostnameVerifier as Checker
import javax.net.ssl.SSLSession

class AliasImportAllowAll : Checker {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
