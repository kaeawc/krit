// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Each call below creates a java.util.Random from the system clock in a file
// that imports javax.crypto, so each is reported. Go matches spelled text and
// misses every one.
package test

import java.lang.System.currentTimeMillis
import java.util.Calendar
import java.util.Date
import java.util.Random
import java.util.Random as JRandom
import javax.crypto.Cipher

typealias SeedRandom = java.util.Random

class Misses {
    // Go misses this because the call is spelled JRandom, not Random.
    fun importAlias(): Random = <!PrngFromSystemTime!>JRandom(System.nanoTime())<!>

    // Go misses this because the call is spelled SeedRandom, not Random.
    fun typeAlias(): Random = <!PrngFromSystemTime!>SeedRandom(System.nanoTime())<!>

    // Go misses this because the backticked name is not spelled Random.
    fun backticked(): Random = <!PrngFromSystemTime!>`Random`(System.nanoTime())<!>

    // Go misses this because the seed text is currentTimeMillis(), without
    // the System. receiver.
    fun staticImport(): Random = <!PrngFromSystemTime!>Random(currentTimeMillis())<!>

    // Go misses this because the parentheses break its `Date().time` text.
    fun parenthesized(): Random = <!PrngFromSystemTime!>Random((Date()).time)<!>

    // Go misses this because the clock read spans lines; Go removes only spaces
    // before matching the seed text.
    fun splitAcrossLines(): Random = <!PrngFromSystemTime!>Random<!>(
        Calendar.getInstance()
            .timeInMillis,
    )

    fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
}
