// Compiler-test source stubs; never packaged in the production artifact.
package android.provider

import android.content.ContentResolver

object Settings {
    const val ACTION_SETTINGS: String = "android.settings.SETTINGS"
    const val ACTION_APPLICATION_DETAILS_SETTINGS: String = "android.settings.APPLICATION_DETAILS_SETTINGS"

    object Global {
        const val AIRPLANE_MODE_ON: String = "airplane_mode_on"

        fun getString(resolver: ContentResolver, name: String): String? = TODO()

        fun getInt(cr: ContentResolver, name: String, def: Int): Int = TODO()

        fun putString(resolver: ContentResolver, name: String, value: String?): Boolean = TODO()
    }

    object Secure {
        const val ANDROID_ID: String = "android_id"

        fun getString(resolver: ContentResolver, name: String): String? = TODO()

        fun getInt(cr: ContentResolver, name: String, def: Int): Int = TODO()
    }

    object System {
        const val SCREEN_BRIGHTNESS: String = "screen_brightness"

        fun getString(resolver: ContentResolver, name: String): String? = TODO()

        fun getInt(cr: ContentResolver, name: String, def: Int): Int = TODO()
    }
}
