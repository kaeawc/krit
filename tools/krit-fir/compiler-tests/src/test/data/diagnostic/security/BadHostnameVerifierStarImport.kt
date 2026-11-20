// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves the SSL wildcard import. Go deliberately requires an explicit
// import or FQN, so unresolved simple names remain silent.
package test

import javax.net.ssl.*

class StarImportAllowAll : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(hostname: String, session: SSLSession): Boolean = true
}
