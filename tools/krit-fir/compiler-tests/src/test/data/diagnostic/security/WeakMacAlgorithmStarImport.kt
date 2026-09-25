// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: a star import of javax.crypto resolves the bare Mac receiver, and
// a companion object named Mac elsewhere in the file does not count as a
// lookalike declaration (Go reports these too).
package test

import javax.crypto.*

class Registry {
    companion object Mac
}

class Crypto {
    fun hash() {
        <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
        Mac.getInstance("HmacSHA256")
    }
}
