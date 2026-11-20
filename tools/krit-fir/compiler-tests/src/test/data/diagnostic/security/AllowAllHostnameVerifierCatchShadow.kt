// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A catch parameter and its references shadow the imported field alias.
package test

import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as verifier

fun caught() {
    try { error("failed") } catch (verifier: Exception) { println(verifier) }
}
