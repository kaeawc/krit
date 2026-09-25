// Smoke: ContextCompat statics, the reified getSystemService, and prefs.edit {}.
package stubs

import android.Manifest
import android.app.NotificationManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.IntentFilter
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.graphics.drawable.Drawable
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.core.content.getSystemService

fun corePermissions(context: Context, prefs: SharedPreferences, receiver: BroadcastReceiver) {
    val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
    val color: Int = ContextCompat.getColor(context, android.R.color.black)
    val drawable: Drawable? = ContextCompat.getDrawable(context, android.R.drawable.ic_menu_add)
    ContextCompat.registerReceiver(context, receiver, IntentFilter("smoke"), ContextCompat.RECEIVER_NOT_EXPORTED)
    val manager: NotificationManager? = context.getSystemService<NotificationManager>()
    val typed: NotificationManager? = ContextCompat.getSystemService(context, NotificationManager::class.java)
    prefs.edit { putString("k", "v") }
    prefs.edit(commit = true) { remove("k") }
    println("$granted $color $drawable $manager $typed")
}
