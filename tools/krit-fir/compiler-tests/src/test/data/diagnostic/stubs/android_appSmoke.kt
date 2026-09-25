// Smoke: subclass platform components and override their real callbacks.
package stubs

import android.app.Activity
import android.app.ActivityManager
import android.app.Application
import android.app.Dialog
import android.app.IntentService
import android.app.Notification
import android.app.NotificationManager
import android.app.Service
import android.app.TabActivity
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.IBinder
import android.view.Menu
import android.view.MenuItem
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
        label.text = intent.getStringExtra("title") ?: ""
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.cancel(1)
        val activityManager: ActivityManager? = getSystemService(ActivityManager::class.java)
        println(activityManager?.isLowRamDevice)
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
