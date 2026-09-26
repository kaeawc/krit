// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// Positive: the verifier interface comes from a star import of the SSL
// package. Go's import facts resolve the star import too, so both report it.
package test

import javax.net.ssl.*

class StarImportAllowAll : HostnameVerifier {
    <!AllowAllHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
