// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate improvement: the file declares a class, object, and typealias
// named Mac, but none of them shadows the javax.crypto.Mac import where the
// calls are made. Go misses every bare `Mac` call here because it skips a bare
// receiver whenever the file declares anything named Mac; FIR is correct to
// report them because each call resolves to javax.crypto.Mac.getInstance.
package test

import javax.crypto.Mac

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
