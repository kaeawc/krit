package fixtures.negative.slacklint

import android.os.Build
import slack.l10n.R as L10nR

fun appName(): Int = L10nR.string.app_name
fun ok(): Int = android.R.string.ok
fun sdk(): Int = Build.VERSION_CODES.R
