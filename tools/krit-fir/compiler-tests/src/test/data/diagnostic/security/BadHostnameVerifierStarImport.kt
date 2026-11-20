// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
// Both engines resolve the SSL wildcard import with no same-file shadow.
package test

import javax.net.ssl.*

class StarImportAllowAll : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
