// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate precision fix: a top-level property named Mac shadows the
// javax.crypto.Mac import everywhere in the file, because a property named Mac
// is found before the class name is tried as the call receiver. Go reports the
// bare call because the receiver is spelled `Mac`, the file imports
// javax.crypto.Mac, and it does not count properties as declarations; FIR is
// correct because the call is MacFactory.getInstance, not
// javax.crypto.Mac.getInstance. The fully qualified JDK call still fires.
package test

import javax.crypto.Mac

class MacFactory {
    fun getInstance(algorithm: String): String = algorithm
}

val Mac = MacFactory()

fun hash() {
    Mac.getInstance("HmacMD5")
    <!WeakMacAlgorithm!>javax.crypto.Mac.getInstance("HmacMD5")<!>
}
