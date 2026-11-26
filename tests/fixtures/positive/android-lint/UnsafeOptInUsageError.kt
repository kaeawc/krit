package fixtures.androidlint

import androidx.annotation.RequiresOptIn

@RequiresOptIn
annotation class ExperimentalSyncError

@ExperimentalSyncError
fun syncNowError() = Unit

fun refreshError() {
    syncNowError()
}
