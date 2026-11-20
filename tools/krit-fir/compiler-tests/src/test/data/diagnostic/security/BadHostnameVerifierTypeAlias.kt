// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 26, 30
// FIR expands all aliases. Go expands parameter aliases but only direct supertype aliases.
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

typealias ChainedSession = Session

class QualifiedString : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: kotlin.String, session: SSLSession) = true
}

class ChainedParameter : HostnameVerifier {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: ChainedSession) = true
}
