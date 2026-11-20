// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
// Both resolve a nested base visible in the same scope. FIR also resolves
// qualified bases; Go's same-file inheritance search is bounded to six levels.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class Scope {
    abstract class Base : HostnameVerifier
    class Nested : Base() {
        <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
    }
}

class Qualified : Scope.Base() {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}

abstract class Root : HostnameVerifier
abstract class First : Root()
abstract class Second : First()
abstract class Third : Second()
abstract class Fourth : Third()
abstract class Fifth : Fourth()
abstract class Sixth : Fifth()
class Deep : Sixth() {
    <!BadHostnameVerifier!>override<!> fun verify(host: String, session: SSLSession) = true
}
