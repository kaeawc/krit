// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 29, 31
// Subclasses: Go matches the spelled call `Random(...)` and the seed text
// `Date().time`, so it reports a Random subclass named Random and a Date
// subclass whose name ends in Date. Each reported call still creates a
// java.util.Random or kotlin.random.Random seeded from the system clock, so
// FIR reports it too.
package test

import java.util.Date
import javax.crypto.Cipher

class Random(seed: Long) : java.util.Random(seed)

object Kotlin {
    class Random(seed: Long) : kotlin.random.Random() {
        private val delegate = kotlin.random.Random(seed)

        override fun nextBits(bitCount: Int): Int = delegate.nextBits(bitCount)
    }
}

class ClockDate : Date()

class Stamp : Date()

fun javaSubclass(): Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>

fun kotlinSubclass(): Kotlin.Random = <!PrngFromSystemTime!>Kotlin.Random(System.nanoTime())<!>

fun dateSubclass(): java.util.Random = <!PrngFromSystemTime!>java.util.Random(ClockDate().time)<!>

// Neither reports this: Go's text is Stamp().time, and FIR follows Go's
// Date-suffix spelling for Date subclasses.
fun otherDateSubclass(): java.util.Random = java.util.Random(Stamp().time)

fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
