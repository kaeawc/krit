// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves inherited Java statics; Go requires an explicit field import/FQN.
package test

import org.apache.http.conn.ssl.SSLSocketFactory

class DerivedFactory : SSLSocketFactory(null) {
    val verifier = <!AllowAllHostnameVerifier!>ALLOW_ALL_HOSTNAME_VERIFIER<!>
}
