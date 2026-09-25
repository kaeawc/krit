// Compiler-test source stubs; never packaged in the production artifact.
package android.icu.text

import java.util.Date
import java.util.Locale

abstract class DateFormat {
    // Declared final on DateFormat, so call sites resolve to DateFormat.format.
    fun format(date: Date): String = TODO()

    open fun parse(text: String): Date? = TODO()
}

open class SimpleDateFormat : DateFormat {
    constructor()

    constructor(pattern: String)

    constructor(pattern: String, locale: Locale)

    open fun toPattern(): String = TODO()
}
