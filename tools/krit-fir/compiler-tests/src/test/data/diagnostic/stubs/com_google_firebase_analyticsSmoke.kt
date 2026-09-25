// Smoke: FirebaseAnalytics.getInstance and logEvent with Event/Param constants.
package stubs

import android.content.Context
import android.os.Bundle
import com.google.firebase.analytics.FirebaseAnalytics

fun track(context: Context) {
    val analytics = FirebaseAnalytics.getInstance(context)
    val params = Bundle()
    params.putString(FirebaseAnalytics.Param.METHOD, "email")
    analytics.logEvent(FirebaseAnalytics.Event.LOGIN, params)
    analytics.logEvent("custom", null)
    analytics.setUserId(null)
    analytics.setAnalyticsCollectionEnabled(false)
}
