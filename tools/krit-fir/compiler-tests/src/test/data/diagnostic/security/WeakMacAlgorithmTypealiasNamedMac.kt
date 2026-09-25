// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvement: a top-level typealias named Mac that expands to
// javax.crypto.Mac, with no import. Go misses the bare `Mac` call because it
// skips a bare receiver whenever the file declares a class, object, or
// typealias named Mac; FIR is correct to report it because the typealias
// resolves to javax.crypto.Mac.getInstance.
package test

typealias Mac = javax.crypto.Mac

class Crypto {
    fun hash() {
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacSHA1")<!>
        Mac.getInstance("HmacSHA256")
    }
}
