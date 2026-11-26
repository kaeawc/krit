package fixtures.positive.slacklint

// Reports only with importAliases configured (slack.l10n.R as L10nR); see
// tests/fixtures/android-lint/WrongResourceImportAlias for the end-to-end fixture.
import slack.l10n.R as Strings

fun appName(): Int = Strings.string.app_name
