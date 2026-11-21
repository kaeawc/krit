// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 9
// Explicit superclass constructor calls are Apache verifier constructions too.
package test

import org.apache.http.conn.ssl.AllowAllHostnameVerifier

class Derived : <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier()<!>
val anonymous = object : <!AllowAllHostnameVerifier!>AllowAllHostnameVerifier()<!> {}

// FIR resolves secondary super calls; Go only visits header constructor calls.
class Secondary : AllowAllHostnameVerifier {
    constructor() : <!AllowAllHostnameVerifier!>super()<!>
}
