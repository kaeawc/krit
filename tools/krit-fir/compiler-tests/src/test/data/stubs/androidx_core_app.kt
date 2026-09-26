// Compiler-test source stubs; never packaged in the production artifact.
package androidx.core.app

import android.app.Activity
import android.app.Notification
import android.content.Context
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner

open class ComponentActivity : Activity(), LifecycleOwner {
    override val lifecycle: Lifecycle
        get() = TODO()
}

// Java class of statics (it extends ContextCompat in Java; static inheritance
// cannot be modeled by an object, so only ActivityCompat's own members exist).
object ActivityCompat {
    fun requestPermissions(activity: Activity, permissions: Array<String>, requestCode: Int) {
        TODO()
    }

    fun shouldShowRequestPermissionRationale(activity: Activity, permission: String): Boolean = TODO()

    fun finishAffinity(activity: Activity) {
        TODO()
    }

    fun recreate(activity: Activity) {
        TODO()
    }
}

object NotificationCompat {
    const val PRIORITY_DEFAULT: Int = 0
    const val PRIORITY_HIGH: Int = 1
    const val PRIORITY_LOW: Int = -1

    open class Builder(context: Context, channelId: String) {
        open fun setSmallIcon(icon: Int): Builder = TODO()

        open fun setContentTitle(title: CharSequence?): Builder = TODO()

        open fun setContentText(text: CharSequence?): Builder = TODO()

        open fun setSubText(text: CharSequence?): Builder = TODO()

        open fun setPriority(pri: Int): Builder = TODO()

        open fun setAutoCancel(autoCancel: Boolean): Builder = TODO()

        open fun setOngoing(ongoing: Boolean): Builder = TODO()

        open fun build(): Notification = TODO()
    }
}

object NotificationManagerCompat {
    fun from(context: Context): NotificationManagerCompat = TODO()

    fun areNotificationsEnabled(): Boolean = TODO()

    fun notify(id: Int, notification: Notification) {
        TODO()
    }

    fun cancel(id: Int) {
        TODO()
    }
}
