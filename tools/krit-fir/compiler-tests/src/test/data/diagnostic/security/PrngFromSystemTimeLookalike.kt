// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 29, 33, 39
// Go reports each call below commented "Go reports this": the seed text
// contains one of its clock substrings. None reads the system clock, so FIR reports nothing: the
// reads are declarations of this file named like the platform ones, a string
// literal, or Date.getTimezoneOffset, which Go's `Date().time` substring also
// matches.
package test

import java.util.Date
import java.util.Random
import javax.crypto.Cipher

object MySystem {
    fun nanoTime(): Long = 7L
}

object Fake {
    class Date {
        val time: Long = 7L
    }
}

class Lookalikes {
    // Go reports this because the seed text contains System.nanoTime().
    fun lookalikeSystem(): Random = Random(MySystem.nanoTime())

    // Go reports this because the seed text contains Date().time.
    fun lookalikeDate(): Random = Random(Fake.Date().time)

    // Go reports this because the seed text contains Date().time.
    @Suppress("DEPRECATION")
    fun timezoneOffset(): Random = Random(Date().timezoneOffset.toLong())

    // Not reported by either: Go's seed node ends before the trailing comment.
    fun comment(): Random = Random(42L /* System.nanoTime() */)

    // Go reports this because the seed text contains System.nanoTime().
    fun stringLiteral(): Random = Random("System.nanoTime()".length.toLong())

    fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
}
