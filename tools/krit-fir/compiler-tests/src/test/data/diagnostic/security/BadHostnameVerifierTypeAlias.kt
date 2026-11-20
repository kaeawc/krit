// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR expands parameter and chained supertype aliases. Go requires the
// parameter type's explicit import/FQN and expands only direct supertype aliases.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

typealias Session = SSLSession
typealias Verifier = HostnameVerifier
typealias ChainedVerifier = Verifier

class AliasedParameter : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: Session) = true
}

class Chained : ChainedVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}

val aliasedSam = <!BadHostnameVerifier!>Verifier { _, _ -> true }<!>
