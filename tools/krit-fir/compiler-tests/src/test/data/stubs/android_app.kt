// Compiler-test source stubs; never packaged in the production artifact.
package android.app

import android.content.Context
import android.content.ContextWrapper
import android.content.DialogInterface
import android.content.Intent
import android.os.Bundle
import android.os.IBinder
import android.view.ContextThemeWrapper
import android.view.Menu
import android.view.MenuInflater
import android.view.MenuItem
import android.view.View

open class Activity : ContextThemeWrapper() {
    // Java getIntent()/setIntent(Intent): unannotated; real Kotlin code
    // dereferences it directly, so it is modeled non-null.
    open var intent: Intent
        get() = TODO()
        set(value) = TODO()

    open val isFinishing: Boolean
        get() = TODO()

    open val menuInflater: MenuInflater
        get() = TODO()

    protected open fun onCreate(savedInstanceState: Bundle?) {
        TODO()
    }

    protected open fun onStart() {
        TODO()
    }

    protected open fun onRestart() {
        TODO()
    }

    protected open fun onResume() {
        TODO()
    }

    protected open fun onPause() {
        TODO()
    }

    protected open fun onStop() {
        TODO()
    }

    protected open fun onDestroy() {
        TODO()
    }

    protected open fun onSaveInstanceState(outState: Bundle) {
        TODO()
    }

    protected open fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        TODO()
    }

    open fun onRequestPermissionsResult(requestCode: Int, permissions: Array<String>, grantResults: IntArray) {
        TODO()
    }

    open fun onBackPressed() {
        TODO()
    }

    open fun onCreateOptionsMenu(menu: Menu): Boolean = TODO()

    open fun onOptionsItemSelected(item: MenuItem): Boolean = TODO()

    open fun setContentView(layoutResID: Int) {
        TODO()
    }

    open fun setContentView(view: View) {
        TODO()
    }

    // Java @Nullable <T extends View> T findViewById(int). The SDK's
    // @RecentlyNullable-style annotation only warns in real Kotlin, and real
    // call sites dereference the result, so it is modeled non-null.
    open fun <T : View> findViewById(id: Int): T = TODO()

    fun <T : View> requireViewById(id: Int): T = TODO()

    open fun finish() {
        TODO()
    }

    fun setResult(resultCode: Int) {
        TODO()
    }

    fun setResult(resultCode: Int, data: Intent?) {
        TODO()
    }

    fun runOnUiThread(action: Runnable) {
        TODO()
    }

    open fun startActivityForResult(intent: Intent, requestCode: Int) {
        TODO()
    }

    fun requestPermissions(permissions: Array<String>, requestCode: Int) {
        TODO()
    }

    // Java statics on a subclassable class live in the companion; their
    // callable ids gain `.Companion` (see README).
    companion object {
        const val RESULT_CANCELED: Int = 0
        const val RESULT_OK: Int = -1
        const val RESULT_FIRST_USER: Int = 1
    }
}

open class ListActivity : Activity()

@Deprecated("Deprecated in Java")
open class ActivityGroup : Activity()

@Deprecated("Deprecated in Java")
open class TabActivity : ActivityGroup()

open class Application : ContextWrapper(null) {
    open fun onCreate() {
        TODO()
    }

    open fun onTerminate() {
        TODO()
    }

    open fun onLowMemory() {
        TODO()
    }
}

abstract class Service : ContextWrapper(null) {
    open fun onCreate() {
        TODO()
    }

    open fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = TODO()

    // Java `@Nullable IBinder onBind(Intent)`; the parameter is unannotated and
    // the Android Studio template overrides it with a non-null Intent.
    abstract fun onBind(intent: Intent): IBinder?

    open fun onUnbind(intent: Intent): Boolean = TODO()

    open fun onDestroy() {
        TODO()
    }

    fun startForeground(id: Int, notification: Notification) {
        TODO()
    }

    fun stopForeground(flags: Int) {
        TODO()
    }

    fun stopSelf() {
        TODO()
    }

    companion object {
        const val START_STICKY: Int = 1
        const val START_NOT_STICKY: Int = 2
        const val START_REDELIVER_INTENT: Int = 3
        const val STOP_FOREGROUND_REMOVE: Int = 1
    }
}

@Deprecated("Deprecated in Java")
abstract class IntentService(name: String) : Service() {
    protected abstract fun onHandleIntent(intent: Intent?)

    override fun onBind(intent: Intent): IBinder? = TODO()

    fun setIntentRedelivery(enabled: Boolean) {
        TODO()
    }
}

open class Dialog(context: Context) : DialogInterface {
    open fun show() {
        TODO()
    }

    override fun cancel() {
        TODO()
    }

    override fun dismiss() {
        TODO()
    }

    open fun setContentView(view: View) {
        TODO()
    }

    open fun setCancelable(flag: Boolean) {
        TODO()
    }
}

open class Notification

// Java classes app code never constructs or subclasses are modeled as objects
// so static members keep their Java callable id (see README).
object NotificationManager {
    const val IMPORTANCE_DEFAULT: Int = 3
    const val IMPORTANCE_HIGH: Int = 4

    fun notify(id: Int, notification: Notification) {
        TODO()
    }

    fun notify(tag: String?, id: Int, notification: Notification) {
        TODO()
    }

    fun cancel(id: Int) {
        TODO()
    }

    fun cancelAll() {
        TODO()
    }

    fun areNotificationsEnabled(): Boolean = TODO()
}

object ActivityManager {
    val isLowRamDevice: Boolean
        get() = TODO()

    fun isUserAMonkey(): Boolean = TODO()
}
