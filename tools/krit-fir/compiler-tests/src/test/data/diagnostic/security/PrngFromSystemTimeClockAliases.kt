// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Each call below seeds a java.util.Random from the system clock in a file
// that imports javax.crypto, so each is reported. Go matches the clock read's
// spelled text (`System.nanoTime()`, `Date().time`, ...) and misses every one.
package test

import java.lang.System as Sys
import java.lang.System.nanoTime as now
import java.time.Instant as I
import java.util.Calendar as Cal
import java.util.Date as D
import java.util.Random
import javax.crypto.Cipher

class Aliases {
    // Go misses this because the seed text is D().time, not Date().time.
    fun dateAlias(): Random = <!PrngFromSystemTime!>Random(D().time)<!>

    // Go misses this because the seed text is Sys.nanoTime(), not
    // System.nanoTime().
    fun systemAlias(): Random = <!PrngFromSystemTime!>Random(Sys.nanoTime())<!>

    // Go misses this because the seed text is Cal.getInstance().timeInMillis.
    fun calendarAlias(): Random = <!PrngFromSystemTime!>Random(Cal.getInstance().timeInMillis)<!>

    // Go misses this because the seed text is I.now().toEpochMilli().
    fun instantAlias(): Random = <!PrngFromSystemTime!>Random(I.now().toEpochMilli())<!>

    // Go misses this because the seed text is now(), an aliased static import.
    fun staticImportAlias(): Random = <!PrngFromSystemTime!>Random(now())<!>

    // Go misses this because the System receiver and nanoTime() sit on
    // different lines; Go removes only spaces before matching.
    fun splitReceiver(): Random = <!PrngFromSystemTime!>Random<!>(java.lang.System
        .nanoTime())

    fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
}
