// RENDER_DIAGNOSTICS_FULL_TEXT
// Recall: the same call spelled in ways Go's text match misses. FIR resolves
// the receiver root and the argument, so it reports all of them; Go reports
// none, because it matches the spelled texts `DateTimeFormatter.ISO_...` and
// `Locale.getDefault()` / `java.util.Locale.getDefault()`.
package lgdffrecall

import java.time.Instant
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeFormatter as Dtf
import java.time.format.DateTimeFormatter.ISO_INSTANT
import java.util.Locale
import java.util.Locale as JavaLocale
import java.util.Locale.getDefault

typealias L = java.util.Locale

// An import alias of DateTimeFormatter.
val aliasedFormatter = <!LocaleGetDefaultForFormatting!>Dtf.ISO_INSTANT.withLocale(Locale.getDefault())<!>

// A static import of the constant.
val staticConstant = <!LocaleGetDefaultForFormatting!>ISO_INSTANT.withLocale(Locale.getDefault())<!>

// An import alias of Locale.
val aliasedLocale = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(JavaLocale.getDefault())<!>

// A type alias of Locale.
val typeAliasedLocale = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(L.getDefault())<!>

// A static import of Locale.getDefault.
val staticGetDefault = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(getDefault())<!>

// A parenthesized receiver and a parenthesized argument.
val parenthesizedReceiver = <!LocaleGetDefaultForFormatting!>(DateTimeFormatter.ISO_INSTANT).withLocale(Locale.getDefault())<!>
val parenthesizedArgument = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale((Locale.getDefault()))<!>

// Comments inside the receiver or the argument: Go compares the text with the
// comments kept.
val blockCommentInReceiver = <!LocaleGetDefaultForFormatting!>DateTimeFormatter /* c */ .ISO_INSTANT.withLocale(Locale.getDefault())<!>
val lineCommentInReceiver = <!LocaleGetDefaultForFormatting!>DateTimeFormatter<!> // c
    .ISO_INSTANT.withLocale(Locale.getDefault())
val commentInArgument = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale /* x */ .getDefault())<!>

// A val holding the constant: Go matches only the constant spelled at the
// root of the receiver; FIR follows the val's initializer.
val isoHolder: DateTimeFormatter = DateTimeFormatter.ISO_INSTANT
val viaTopLevelVal = <!LocaleGetDefaultForFormatting!>isoHolder.withLocale(Locale.getDefault())<!>

fun viaLocalVal(instant: Instant): String {
    val formatter = DateTimeFormatter.ISO_INSTANT
    return <!LocaleGetDefaultForFormatting!>formatter.withLocale(Locale.getDefault())<!>.format(instant)
}
