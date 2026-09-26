// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for ImplicitDefaultLocale that Go leaves alone too.
package test

import java.util.Locale

@Suppress("DEPRECATION_ERROR")
class ExplicitCaseConversions {
    fun lower(s: String): String = s.toLowerCase(Locale.ROOT)

    fun upper(s: String): String = s.toUpperCase(Locale.getDefault())

    // Kotlin 1.5+ lowercase() / uppercase() are locale-invariant.
    fun invariantLower(s: String): String = s.lowercase()

    fun invariantUpper(s: String): String = s.uppercase()

    // Go's ASCII-invariant receivers: the receiver text contains one of its
    // identifiers (a substring match, so `ghostName` matches `host`).
    fun currency(currencyCode: String): String = currencyCode.toUpperCase()

    fun mime(response: Response): String = response.mimeType.toLowerCase()

    fun ghost(ghostName: String): String = ghostName.toLowerCase()

    fun hexDigits(value: Int): String = Integer.toHexString(value).toUpperCase()

    fun capitalizedMethod(method: String): String = method.capitalize()

    // No explicit receiver: Go only visits qualified calls.
    fun implicitReceiver(s: String): String = with(s) { toLowerCase() }

    fun String.extensionBody(): String = toUpperCase()
}

class Response(val mimeType: String)

class ExplicitFormat {
    fun static(value: Int): String = String.format(Locale.US, "%d", value)

    fun staticRoot(value: Double): String = String.format(Locale.ROOT, "%.2f", value)

    fun instance(value: Int): String = "%d".format(Locale.US, value)

    fun constructed(value: Int): String = String.format(Locale("en", "US"), "%d", value)
}

const val NAME_FORMAT = "%s: %x"
const val RAW_FORMAT = """"%s""""
const val ESCAPED_FORMAT = "\t%s\n"

object Formats {
    const val LABEL = "[%s]"
}

class LocaleInsensitiveFormat {
    fun strings(name: String): String = String.format("%s", name)

    fun percent(): String = String.format("progress %% done%n")

    fun hex(value: Int): String = String.format("%08x", value)

    fun booleanAndChar(flag: Boolean, c: Char): String = String.format("%b %c %h %o", flag, c, c, 8)

    fun constant(name: String, value: Int): String = String.format(NAME_FORMAT, name, value)

    fun qualifiedConstant(name: String): String = String.format(Formats.LABEL, name)

    fun parenthesizedConstant(name: String): String = String.format((NAME_FORMAT), name, 1)

    fun localConstant(name: String): String {
        val local = "<%S>"
        return String.format(local, name)
    }

    fun rawConstant(name: String): String = String.format(RAW_FORMAT, name)

    fun escapedConstant(name: String): String = String.format(ESCAPED_FORMAT, name)

    fun instance(name: String): String = "%s".format(name)

    fun instanceHex(value: Int): String = "%X".format(value)

    fun noConversions(): String = "%%d".format()

    fun rawInstance(name: String): String = """[%s]""".format(name)
}

// A receiver that is not a string literal: Go only considers a literal or the
// text `String`.
class NonLiteralReceiver {
    fun pattern(pattern: String, value: Int): String = pattern.format(value)

    fun parenthesized(value: Int): String = ("%d").format(value)
}
