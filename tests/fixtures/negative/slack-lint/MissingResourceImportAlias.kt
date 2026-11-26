package fixtures.negative.slacklint.feature

// The module's own R class needs no alias.
import fixtures.negative.slacklint.R
import slack.l10n.R as L10nR

fun appName(): Int = R.string.local + L10nR.string.app_name
