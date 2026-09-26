// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvement: an import alias of the verifier interface. Go
// misses it because the class header does not spell HostnameVerifier.
package test

import javax.net.ssl.HostnameVerifier as Checker
import javax.net.ssl.SSLSession

class AliasImportAllowAll : Checker {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
