// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 25
// Go's `Date().time` seed substring also matches `toDate().time` and the
// call of any function whose name ends in Date. FIR reports only a Date of
// the current instant; the Joda-Time `DateTime().toDate().time` positives are
// in PrngFromSystemTimeFilesTest, since the stubs declare no Joda-Time.
package test

import java.util.Date
import java.util.Random
import javax.crypto.Cipher

class Event(private val at: Long) {
    fun toDate(): Date = Date(at)
}

fun startDate(): Date = Date(0L)

// Go reports this because the seed text `event.toDate().time` contains
// `Date().time`. The seed is the event's own time, not the system time.
fun eventSeed(event: Event): Random = Random(event.toDate().time)

// Go reports this because the seed text `startDate().time` contains
// `Date().time`. The seed is the epoch, not the system time.
fun startSeed(): Random = Random(startDate().time)

// Neither reports this: Go's text is `event.toDate().hashCode()`.
fun hashSeed(event: Event): Random = Random(event.toDate().hashCode().toLong())

fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
