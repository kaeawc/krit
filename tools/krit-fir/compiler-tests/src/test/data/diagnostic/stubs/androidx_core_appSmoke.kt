// Smoke: core ComponentActivity supertypes, ActivityCompat permissions, NotificationCompat.
package stubs

import android.Manifest
import android.app.Activity
import android.app.Notification
import android.content.Context
import androidx.core.app.ActivityCompat
import androidx.core.app.ComponentActivity
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.lifecycle.LifecycleOwner

fun askCamera(activity: Activity) {
    if (ActivityCompat.shouldShowRequestPermissionRationale(activity, Manifest.permission.CAMERA)) return
    ActivityCompat.requestPermissions(activity, arrayOf(Manifest.permission.CAMERA), 42)
}

fun coreHierarchy(activity: ComponentActivity) {
    val owner: LifecycleOwner = activity
    val platform: Activity = activity
    <!PrintlnInProduction!>println<!>("$owner $platform")
}

@Suppress("MissingPermission")
fun postCompat(context: Context) {
    val notification: Notification = NotificationCompat.Builder(context, "channel")
        .setSmallIcon(android.R.drawable.ic_menu_add)
        .setContentTitle("Title")
        .setContentText("Body")
        .setSubText("Summary")
        .setPriority(NotificationCompat.PRIORITY_DEFAULT)
        .setAutoCancel(true)
        .build()
    val manager = NotificationManagerCompat.from(context)
    if (manager.areNotificationsEnabled()) manager.notify(1, notification)
}
