// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: Random without a clock-derived seed, a seed whose clock read is
// out of reach of Go's text match (a variable, a callable reference), clock
// calls that take arguments, and supertype delegations, which are not call
// expressions.
package test

import java.security.SecureRandom
import java.time.Clock
import java.time.Instant
import java.util.Calendar
import java.util.Date
import java.util.Random
import java.util.TimeZone

class Negatives(private val clock: Clock) {
    fun unseeded(): Random = Random()

    fun fixed(): Random = Random(42L)

    fun parameter(seed: Long): Random = Random(seed)

    fun secure(): SecureRandom = SecureRandom()

    fun viaVariable(): Random {
        val now = System.nanoTime()
        return Random(now)
    }

    fun callableReference(): Random = Random(run(System::nanoTime))

    fun epochDate(): Random = Random(Date(0L).time)

    fun zonedCalendar(): Random = Random(Calendar.getInstance(TimeZone.getDefault()).timeInMillis)

    fun clockInstant(): Random = Random(Instant.now(clock).toEpochMilli())

    fun epoch(): Random = Random(Instant.EPOCH.toEpochMilli())

    fun dateHash(): Random = Random(Date().hashCode().toLong())
}

class SeededRandom : Random(System.nanoTime())

fun anonymous(): Random = object : Random(System.nanoTime()) {}
