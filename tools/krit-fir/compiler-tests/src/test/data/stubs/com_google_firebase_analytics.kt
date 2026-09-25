// Compiler-test source stubs; never packaged in the production artifact.
package com.google.firebase.analytics

import android.content.Context
import android.os.Bundle

// Java final class obtained through the static getInstance(Context).
object FirebaseAnalytics {
    fun getInstance(context: Context): FirebaseAnalytics = TODO()

    fun logEvent(name: String, params: Bundle?) {
        TODO()
    }

    fun setUserId(id: String?) {
        TODO()
    }

    fun setUserProperty(name: String, value: String?) {
        TODO()
    }

    fun setAnalyticsCollectionEnabled(enabled: Boolean) {
        TODO()
    }

    object Event {
        const val LOGIN: String = "login"
        const val SCREEN_VIEW: String = "screen_view"
        const val SELECT_CONTENT: String = "select_content"
        const val SIGN_UP: String = "sign_up"
    }

    object Param {
        const val ITEM_ID: String = "item_id"
        const val METHOD: String = "method"
        const val SCREEN_NAME: String = "screen_name"
    }
}
