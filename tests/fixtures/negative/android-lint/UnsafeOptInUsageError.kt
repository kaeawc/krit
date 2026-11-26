package fixtures.androidlint

import androidx.annotation.OptIn
import androidx.annotation.RequiresOptIn

@RequiresOptIn
annotation class ExperimentalSyncError

@ExperimentalSyncError
fun syncNowError() = Unit

@OptIn(markerClass = [ExperimentalSyncError::class])
fun refreshError() {
    syncNowError()
}
