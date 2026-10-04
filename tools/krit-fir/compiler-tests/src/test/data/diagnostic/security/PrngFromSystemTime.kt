// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 18, 20, 22, 24, 26, 28, 30, 32, 35, 37, 39, 41, 43, 45, 47, 50, 51, 54, 56, 59, 65, 67, 70, 74, 80
// Positive: a java.util.Random created with a seed that reads the system clock,
// in a file that imports javax.crypto, java.security, or javax.net.ssl, is
// reported on the line where the call expression starts. The clock read may
// sit anywhere inside the seed, as Go matches a substring of the seed text.
package test

import java.security.SecureRandom
import java.time.Instant
import java.util.Calendar
import java.util.Date
import java.util.Random

class Seeds {
    fun millis(): Random = <!PrngFromSystemTime!>Random(System.currentTimeMillis())<!>

    fun nanos(): Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>

    fun date(): Random = <!PrngFromSystemTime!>Random(Date().time)<!>

    fun dateGetter(): Random = <!PrngFromSystemTime!>Random(Date().getTime())<!>

    fun calendar(): Random = <!PrngFromSystemTime!>Random(Calendar.getInstance().timeInMillis)<!>

    fun calendarGetter(): Random = <!PrngFromSystemTime!>Random(Calendar.getInstance().getTimeInMillis())<!>

    fun instant(): Random = <!PrngFromSystemTime!>Random(Instant.now().toEpochMilli())<!>

    fun qualified(): Random = <!PrngFromSystemTime!>java.util.Random(java.lang.System.nanoTime())<!>

    fun qualifiedDate(): Random = <!PrngFromSystemTime!>Random(java.util.Date().time)<!>

    // GregorianCalendar.getInstance() is Calendar.getInstance().
    fun gregorian(): Random = <!PrngFromSystemTime!>Random(java.util.GregorianCalendar.getInstance().timeInMillis)<!>

    fun mixed(): Random = <!PrngFromSystemTime!>Random(System.nanoTime() xor 0x5DEECE66DL)<!>

    fun converted(): Random = <!PrngFromSystemTime!>Random(System.currentTimeMillis().hashCode().toLong())<!>

    fun spaced(): Random = <!PrngFromSystemTime!>Random(System . nanoTime( ))<!>

    fun template(): Random = <!PrngFromSystemTime!>Random("${System.nanoTime()}".hashCode().toLong())<!>

    fun inLambda(): Random = <!PrngFromSystemTime!>Random(run { System.nanoTime() })<!>

    fun conditional(fixed: Boolean): Random = <!PrngFromSystemTime!>Random(if (fixed) 42L else System.nanoTime())<!>

    // Both calls are reported: the outer seed contains the inner call.
    fun nested(): Random = <!PrngFromSystemTime!>Random<!>(
        <!PrngFromSystemTime!>Random(System.nanoTime())<!>.nextLong(),
    )

    fun chained(): Int = <!PrngFromSystemTime!>Random(System.nanoTime())<!>.nextInt()

    val property: Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>

    fun multiLine(): Random {
        val random = <!PrngFromSystemTime!>Random<!>(
            System.nanoTime(),
        )
        return random
    }

    fun inLambdaBody(): () -> Random = { <!PrngFromSystemTime!>Random(System.nanoTime())<!> }

    fun defaultArgument(random: Random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>): Random = random

    fun inObjectLiteral(): Any = object {
        val random = <!PrngFromSystemTime!>Random(System.nanoTime())<!>
    }

    companion object {
        val shared = <!PrngFromSystemTime!>Random(System.nanoTime())<!>
    }

    fun secure(): SecureRandom = SecureRandom()
}

fun topLevel(): Random = <!PrngFromSystemTime!>Random(System.currentTimeMillis())<!>
