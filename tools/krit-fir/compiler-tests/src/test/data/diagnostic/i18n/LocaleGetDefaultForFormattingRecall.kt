// RENDER_DIAGNOSTICS_FULL_TEXT
// Recall: the same call spelled in ways Go's text match misses. FIR resolves
// the receiver root and the argument, so it reports all of them; Go reports
// none, because it matches the spelled texts `DateTimeFormatter.ISO_...` and
// `Locale.getDefault()` / `java.util.Locale.getDefault()`.
package lgdffrecall

import java.time.format.DateTimeFormatter
import java.time.format.DateTimeFormatter as Dtf
import java.time.format.DateTimeFormatter.ISO_INSTANT
import java.util.Locale
import java.util.Locale as JavaLocale
import java.util.Locale.getDefault

// An import alias of DateTimeFormatter.
val aliasedFormatter = <!LocaleGetDefaultForFormatting!>Dtf.ISO_INSTANT.withLocale(Locale.getDefault())<!>

// A static import of the constant.
val staticConstant = <!LocaleGetDefaultForFormatting!>ISO_INSTANT.withLocale(Locale.getDefault())<!>

// An import alias of Locale.
val aliasedLocale = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(JavaLocale.getDefault())<!>

// A static import of Locale.getDefault.
val staticGetDefault = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(getDefault())<!>

// A parenthesized receiver and a parenthesized argument.
val parenthesizedReceiver = <!LocaleGetDefaultForFormatting!>(DateTimeFormatter.ISO_INSTANT).withLocale(Locale.getDefault())<!>
val parenthesizedArgument = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale((Locale.getDefault()))<!>
