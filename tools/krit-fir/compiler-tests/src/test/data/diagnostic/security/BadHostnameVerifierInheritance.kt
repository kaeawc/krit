// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves nested, qualified and deep bases. Go's class-wide Lint visitor
// skips these superclass constructor shapes; its inheritance lookup is bounded.
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
