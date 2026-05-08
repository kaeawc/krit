package test

import java.time.Instant
import java.time.format.DateTimeFormatter
import java.util.Locale

fun isoTimestamp(instant: Instant): String =
    DateTimeFormatter.ISO_INSTANT
        .withLocale(Locale.ROOT)
        .format(instant)
