// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Enum entry declarations and references are not uses of the imported field.
package test

import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER

enum class VerifierPolicy { ALLOW_ALL_HOSTNAME_VERIFIER, STRICT }
val policy = VerifierPolicy.ALLOW_ALL_HOSTNAME_VERIFIER
