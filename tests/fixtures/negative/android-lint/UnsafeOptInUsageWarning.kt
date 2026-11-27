package fixtures.androidlint

import androidx.annotation.OptIn
import androidx.annotation.RequiresOptIn

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class ExperimentalSyncWarning

@ExperimentalSyncWarning
fun syncNowWarning() = Unit

@OptIn(markerClass = [ExperimentalSyncWarning::class])
fun refreshWarning() {
    syncNowWarning()
}
