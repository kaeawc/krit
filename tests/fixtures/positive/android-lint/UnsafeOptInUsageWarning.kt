package fixtures.androidlint

import androidx.annotation.RequiresOptIn

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class ExperimentalSyncWarning

@ExperimentalSyncWarning
fun syncNowWarning() = Unit

fun refreshWarning() {
    syncNowWarning()
}
