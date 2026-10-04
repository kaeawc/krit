// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 25, 29, 30, 38, 39, 49, 50, 56
// Deliberate precision fix: a local val, a parameter, a member property, and a
// companion object named Mac shadow the javax.crypto.Mac import, so
// `Mac.getInstance(...)` below calls the lookalike's getInstance, not
// javax.crypto.Mac's. Go reports every shadowed call because the receiver is
// spelled `Mac`, the file imports javax.crypto.Mac, and it does not count
// locals, parameters, properties, or companions as declarations; FIR is correct
// because none of these calls is javax.crypto.Mac.getInstance. The fully
// qualified JDK calls still fire. A top-level property named Mac is pinned in
// WeakMacAlgorithmShadowedTopLevel instead, because it would shadow the import
// across this whole file.
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

class MemberShadow {
    val Mac = MacFactory()

    fun hash() {
        Mac.getInstance("HmacMD5")
        <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
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
