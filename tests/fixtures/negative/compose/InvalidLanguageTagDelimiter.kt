package fixtures.compose

import androidx.compose.ui.text.intl.Locale

val UsEnglish = Locale("en-US")
val Platform = java.util.Locale("en_US")

fun tagFor(region: String) = Locale("en_$region")
