// Smoke: @ApplicationContext / @ActivityContext qualifiers on injected parameters.
package stubs

import android.content.Context
import dagger.hilt.android.qualifiers.ActivityContext
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

class ContextHolder @Inject constructor(
    @ApplicationContext private val appContext: Context,
    @ActivityContext private val activityContext: Context,
) {
    fun label(): String = appContext.getString(android.R.string.ok) + activityContext.packageName
}
