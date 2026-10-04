// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// A star import of java.time.format: Go's import gate accepts the star import
// of the formatter's package, and FIR resolves the java.time ISO constant, so
// both report the call.
package lgdffstarimport

import java.time.format.*
import java.util.Locale

val starImported = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())<!>
