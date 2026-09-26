// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 17, 19, 21, 23, 25, 27, 30, 36, 39, 43, 45, 48, 61, 63, 65, 67, 69, 71, 73, 76, 79, 82, 87, 92, 94, 98, 100, 102, 104, 106, 108, 111x2
// Positives for ImplicitDefaultLocale, each reported by the Go rule too: the
// no-argument String case conversions, String.format without a Locale, and
// "pattern".format(...) on a string literal, with a locale-sensitive pattern.
// K2 rejects the stdlib toLowerCase() / toUpperCase() / capitalize() /
// decapitalize() as DEPRECATION_ERROR, so those calls only compile with the
// error suppressed. Like Go, the finding sits on the first line of the call.
package test

import java.util.Date

@Suppress("DEPRECATION_ERROR")
class CaseConversions {
    fun lower(s: String): String = <!ImplicitDefaultLocale!>s.toLowerCase()<!>

    fun upper(s: String): String = <!ImplicitDefaultLocale!>s.toUpperCase()<!>

    fun capitalized(s: String): String = <!ImplicitDefaultLocale!>s.capitalize()<!>

    fun decapitalized(s: String): String = <!ImplicitDefaultLocale!>s.decapitalize()<!>

    fun literal(): String = <!ImplicitDefaultLocale!>"ABC".toLowerCase()<!>

    fun safeCall(s: String?): String? = <!ImplicitDefaultLocale!>s?.toLowerCase()<!>

    fun chained(s: String): String = <!ImplicitDefaultLocale!>s.trim().toUpperCase()<!>

    fun splitReceiver(s: String): String {
        val text = <!ImplicitDefaultLocale!>s<!>
            .trim()
            .toLowerCase()
        return text
    }

    fun inLambda(values: List<String>): List<String> = values.map { <!ImplicitDefaultLocale!>it.toLowerCase()<!> }

    // A platform-typed String from a Java API.
    fun platform(): String = <!ImplicitDefaultLocale!>System.getProperty("user.name").toUpperCase()<!>

    // The java.lang.String member, not the stdlib extension.
    @Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
    fun javaMember(s: String): String = <!ImplicitDefaultLocale!>(s as java.lang.String).toLowerCase()<!>

    fun String.shout(): String = <!ImplicitDefaultLocale!>this.toUpperCase()<!>

    fun inObject(): Any = object {
        fun lower(s: String): String = <!ImplicitDefaultLocale!>s.toLowerCase()<!>
    }
}

const val NUMBER_FORMAT = "%d"
val TEMPLATE_FORMAT = "${NUMBER_FORMAT}!"
const val POSITIONAL = "%1\$s"

object Fmt {
    const val PAIR = "%1\$s=%2\$s"
}

class StaticFormat {
    fun decimal(value: Double): String = <!ImplicitDefaultLocale!>String.format("%.2f", value)<!>

    fun grouped(value: Int): String = <!ImplicitDefaultLocale!>String.format("%,d", value)<!>

    fun date(): String = <!ImplicitDefaultLocale!>String.format("%tF", Date())<!>

    fun padded(value: Int): String = <!ImplicitDefaultLocale!>String.format("%08d", value)<!>

    fun positional(value: String): String = <!ImplicitDefaultLocale!>String.format("%1\$s", value)<!>

    fun patternFromParameter(pattern: String, value: Int): String = <!ImplicitDefaultLocale!>String.format(pattern, value)<!>

    fun sensitiveConstant(value: Int): String = <!ImplicitDefaultLocale!>String.format(NUMBER_FORMAT, value)<!>

    // A template initializer is not a plain literal, so its pattern is unknown.
    fun templateConstant(value: Int): String = <!ImplicitDefaultLocale!>String.format(TEMPLATE_FORMAT, value)<!>

    // A named first argument is not a string literal as written.
    fun namedPattern(value: String): String = <!ImplicitDefaultLocale!>String.format(format = "%s", value)<!>

    fun splitReceiver(value: Int): String {
        val text = <!ImplicitDefaultLocale!>String<!>
            .format("%d", value)
        return text
    }

    fun nested(value: Int): String = <!ImplicitDefaultLocale!>String.format("%s", String.format("%d", value))<!>

    // A constant's pattern is read as written, escapes included, as Go reads
    // it: `%1\$s` is positional, which Go treats as locale-sensitive, the same
    // as the literal in `positional` above.
    fun positionalConstant(value: String): String = <!ImplicitDefaultLocale!>String.format(POSITIONAL, value)<!>

    fun positionalQualifiedConstant(a: String, b: String): String = <!ImplicitDefaultLocale!>String.format(Fmt.PAIR, a, b)<!>
}

class InstanceFormat {
    fun number(value: Int): String = <!ImplicitDefaultLocale!>"%d".format(value)<!>

    fun decimal(value: Double): String = <!ImplicitDefaultLocale!>"%.2f".format(value)<!>

    fun timestamp(): String = <!ImplicitDefaultLocale!>"Timestamp: %d".format(System.currentTimeMillis())<!>

    fun template(prefix: String, value: Int): String = <!ImplicitDefaultLocale!>"$prefix %d".format(value)<!>

    fun raw(value: Int): String = <!ImplicitDefaultLocale!>"""%d""".format(value)<!>

    fun mixed(name: String, value: Int): String = <!ImplicitDefaultLocale!>"%s %d".format(name, value)<!>

    @Suppress("DEPRECATION_ERROR")
    fun chained(value: Int): String = <!ImplicitDefaultLocale!>"%d".format(value).toUpperCase()<!>
}
