// Smoke: Settings.Global / Settings.Secure static lookups.
package stubs

import android.content.Context
import android.content.Intent
import android.provider.Settings

fun deviceSettings(context: Context): String? {
    val resolver = context.contentResolver
    val airplane: Int = Settings.Global.getInt(resolver, Settings.Global.AIRPLANE_MODE_ON, 0)
    val id: String? = Settings.Secure.getString(resolver, Settings.Secure.ANDROID_ID)
    context.startActivity(Intent(Settings.ACTION_SETTINGS))
    return Settings.Global.getString(resolver, "name") ?: "$airplane$id"
}
