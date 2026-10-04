// Smoke: subclass platform components and override their real callbacks.
package stubs

import android.app.Activity
import android.app.ActivityManager
import android.app.AlarmManager
import android.app.Application
import android.app.Dialog
import android.app.DialogFragment
import android.app.Fragment
import android.app.IntentService
import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.app.TabActivity
import android.content.Context
import android.content.DialogInterface
import android.content.Intent
import android.os.Bundle
import android.os.IBinder
import android.view.LayoutInflater
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.view.ViewGroup
import android.widget.TextView

class SmokeApplication : Application() {
    override fun onCreate() {
        super.onCreate()
    }
}

class SmokeActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(android.R.layout.simple_list_item_1)
        val label = findViewById<TextView>(android.R.id.content)
        // getStringExtra returns a platform type, so it assigns straight into text.
        label.text = intent.getStringExtra("title")
        startActivity(Intent(Intent.ACTION_VIEW))
        setResult(Activity.RESULT_OK)
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.cancel(1)
        val activityManager: ActivityManager? = getSystemService(ActivityManager::class.java)
        <!PrintlnInProduction!>println<!>(activityManager?.isLowRamDevice)
        startActivity(Intent(this, SmokeActivity::class.java))
        runOnUiThread { Dialog(this).show() }
        if (isFinishing) finish()
    }

    override fun onResume() {
        super.onResume()
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean = super.onCreateOptionsMenu(menu)

    override fun onOptionsItemSelected(item: MenuItem): Boolean = super.onOptionsItemSelected(item)

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (resultCode == Activity.RESULT_OK) setResult(Activity.RESULT_CANCELED)
    }
}

class SmokeService : Service() {
    override fun onBind(intent: Intent): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = START_STICKY

    override fun onDestroy() {
        stopSelf()
        super.onDestroy()
    }
}

@Suppress("DEPRECATION")
class SmokeIntentService : IntentService("smoke") {
    @Deprecated("Deprecated in Java")
    override fun onHandleIntent(intent: Intent?) {}
}

@Suppress("DEPRECATION")
class SmokeTabActivity : TabActivity()

fun postNotification(manager: NotificationManager, notification: Notification) {
    manager.notify(7, notification)
}

fun openAppIntent(context: Context): PendingIntent {
    val intent = Intent(context, SmokeActivity::class.java)
    return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
}

fun backStackIntent(context: Context, options: Bundle?): PendingIntent {
    val intents = arrayOf(Intent(context, SmokeActivity::class.java), Intent(Intent.ACTION_VIEW))
    val single = PendingIntent.getActivity(context, 1, intents[0], PendingIntent.FLAG_IMMUTABLE, options)
    single.cancel()
    PendingIntent.getActivities(context, 2, intents, PendingIntent.FLAG_IMMUTABLE, options)
    return PendingIntent.getActivities(context, 3, intents, PendingIntent.FLAG_IMMUTABLE)
}

// Repeating alarms at the platform's inexact intervals; exact alarms gated on
// the API 31 permission check.
fun scheduleSync(context: Context) {
    val alarms = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
    val intent = Intent(context, SmokeService::class.java)
    val operation = PendingIntent.getForegroundService(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
    val start = System.currentTimeMillis() + AlarmManager.INTERVAL_HOUR
    alarms.setInexactRepeating(AlarmManager.RTC_WAKEUP, start, AlarmManager.INTERVAL_FIFTEEN_MINUTES, operation)
    alarms.setRepeating(AlarmManager.ELAPSED_REALTIME, start, AlarmManager.INTERVAL_DAY, operation)
    if (alarms.canScheduleExactAlarms()) {
        alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, start, operation)
    } else {
        alarms.setWindow(AlarmManager.RTC, start, AlarmManager.INTERVAL_HALF_HOUR, operation)
    }
    alarms.cancel(operation)
}

// Framework fragments (deprecated in API 28) keep their no-arg constructors.
@Suppress("DEPRECATION")
class SmokePlatformFragment : Fragment() {
    @Deprecated("Deprecated in Java")
    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View? =
        inflater.inflate(android.R.layout.simple_list_item_1, container, false)
}

@Suppress("DEPRECATION")
class SmokePlatformDialogFragment : DialogFragment() {
    @Deprecated("Deprecated in Java")
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog = Dialog(activity)

    @Deprecated("Deprecated in Java")
    override fun onDismiss(dialog: DialogInterface) {
        super.onDismiss(dialog)
    }
}
