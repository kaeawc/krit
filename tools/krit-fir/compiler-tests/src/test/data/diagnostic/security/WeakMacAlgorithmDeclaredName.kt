// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33
// Deliberate improvement: the file declares a top-level class, a nested class,
// a nested object, and a local class named Mac, but none of them shadows the
// javax.crypto.Mac import where the calls are made. The explicit import wins
// over the same-package top-level class, and the nested and local classes are
// out of scope in Crypto. Go misses every bare `Mac` call here because it skips
// a bare receiver whenever the file declares anything named Mac; FIR is correct
// to report them because each call resolves to javax.crypto.Mac.getInstance.
package test

import javax.crypto.Mac

class Mac

class Holder {
    class Mac
}

object Registry {
    object Mac
}

class Aliases {
    fun local() {
        class Mac
    }
}

class Crypto {
    fun hash() {
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
        Mac.getInstance("HmacSHA256")
    }
}
