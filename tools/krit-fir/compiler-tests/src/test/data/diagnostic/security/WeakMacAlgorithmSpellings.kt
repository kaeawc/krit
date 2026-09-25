// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: other spellings of javax.crypto.Mac.getInstance.
// Go misses every call below because it only accepts a receiver spelled `Mac`
// or `javax.crypto.Mac`; FIR is correct to report them because each call
// resolves to javax.crypto.Mac.getInstance with a weak HMAC literal.
package test

import javax.crypto.Mac as HmacFactory
import javax.crypto.Mac.getInstance
import javax.crypto.Mac.getInstance as macFor

typealias AliasedMac = javax.crypto.Mac

class Crypto {
    fun hash() {
        // Import alias of javax.crypto.Mac.
        <!WeakMacAlgorithm!>HmacFactory.getInstance("HmacMD5")<!>
        // Typealias of javax.crypto.Mac.
        <!WeakMacAlgorithm!>AliasedMac.getInstance("HmacSHA1")<!>
        // Statically imported getInstance, plain and aliased.
        <!WeakMacAlgorithm!>getInstance("HmacMD5")<!>
        <!WeakMacAlgorithm!>macFor("HmacSHA1")<!>
        // Backticked receiver.
        <!WeakMacAlgorithm!>`HmacFactory`.getInstance("HmacMD5")<!>

        HmacFactory.getInstance("HmacSHA256")
        AliasedMac.getInstance("HmacSHA512")
        getInstance("HmacSHA256")
        macFor("HmacSHA384")
    }
}
