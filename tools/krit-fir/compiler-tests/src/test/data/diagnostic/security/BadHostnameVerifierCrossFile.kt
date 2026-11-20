// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves the base declared in a separate stub file. Go cannot establish
// the imported base's supertype from this source file and stays silent.
package test

import fixture.hostname.VerifierBase
import javax.net.ssl.SSLSession

class Derived : VerifierBase() {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}
