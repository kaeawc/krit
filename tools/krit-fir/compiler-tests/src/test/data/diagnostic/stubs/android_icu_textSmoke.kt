// Smoke: ICU SimpleDateFormat construction and DateFormat.format.
package stubs

import android.icu.text.DateFormat
import android.icu.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

fun formatToday(): String {
    val format: DateFormat = SimpleDateFormat("yyyy-MM-dd", Locale.US)
    // Pass a locale: SimpleDateFormat(pattern) uses the default locale, which
    // the SimpleDateFormat rule reports.
    val plain = SimpleDateFormat("HH:mm", Locale.ROOT)
    return format.format(Date()) + plain.format(Date())
}
