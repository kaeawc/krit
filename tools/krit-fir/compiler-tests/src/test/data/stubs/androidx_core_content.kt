// Compiler-test source stubs; never packaged in the production artifact.
package androidx.core.content

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.SharedPreferences
import android.graphics.drawable.Drawable
import java.util.concurrent.Executor

object ContextCompat {
    const val RECEIVER_VISIBLE_TO_INSTANT_APPS: Int = 1
    const val RECEIVER_EXPORTED: Int = 2
    const val RECEIVER_NOT_EXPORTED: Int = 4

    fun checkSelfPermission(context: Context, permission: String): Int = TODO()

    fun getColor(context: Context, id: Int): Int = TODO()

    fun getDrawable(context: Context, id: Int): Drawable? = TODO()

    fun <T> getSystemService(context: Context, serviceClass: Class<T>): T? = TODO()

    fun startForegroundService(context: Context, intent: Intent) {
        TODO()
    }

    fun registerReceiver(context: Context, receiver: BroadcastReceiver?, filter: IntentFilter, flags: Int): Intent? =
        TODO()

    fun getMainExecutor(context: Context): Executor = TODO()
}

inline fun <reified T : Any> Context.getSystemService(): T? = TODO()

inline fun SharedPreferences.edit(commit: Boolean = false, action: SharedPreferences.Editor.() -> Unit) {
    TODO()
}
