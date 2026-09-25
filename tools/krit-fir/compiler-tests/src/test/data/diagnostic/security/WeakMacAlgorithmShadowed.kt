// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate precision fix: a local val, a parameter, and a companion object
// named Mac shadow the javax.crypto.Mac import, so `Mac.getInstance(...)` below
// calls the lookalike's getInstance, not javax.crypto.Mac's. Go reports every
// shadowed call because the receiver is spelled `Mac`, the file imports
// javax.crypto.Mac, and it does not count locals, parameters, or companions as
// declarations; FIR is correct because none of these calls is
// javax.crypto.Mac.getInstance. The fully qualified JDK calls still fire.
package test

import javax.crypto.Mac

class MacFactory {
    fun getInstance(algorithm: String): String = algorithm
}

class LocalShadow {
    fun hash() {
        val Mac = MacFactory()
        Mac.getInstance("HmacMD5")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
    }

    fun parameter(Mac: MacFactory) {
        Mac.getInstance("HmacSHA1")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacSHA1")<!>
    }
}

class CompanionShadow {
    companion object Mac {
        fun getInstance(algorithm: String): String = algorithm
    }

    fun hash() {
        Mac.getInstance("HmacMD5")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
    }
}

// Outside the shadowing scopes the import resolves again.
fun unshadowed() {
    <!WeakMacAlgorithm!>Mac.getInstance("HmacMD5")<!>
}
