// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 29, 54
// Android Lint's onDestroy check for Wakelock: a release in an Activity's
// onDestroy runs too late, so it is reported on the release. Lambdas and
// local functions sit inside the method they belong to; a release in another
// method, in a class that is not an Activity, or in a member of an anonymous
// object is not reported.
package test

import android.app.Activity
import android.app.Service
import android.content.Intent
import android.os.IBinder
import android.os.PowerManager.WakeLock

class PlayerActivity : Activity() {
    private var wakeLock: WakeLock? = null

    override fun onResume() {
        super.onResume()
        wakeLock?.acquire()
    }

    override fun onDestroy() {
        super.onDestroy()
        <!Wakelock!>wakeLock?.release()<!>
        run { <!Wakelock!>wakeLock?.release()<!> }
        fun cleanup() {
            <!Wakelock!>wakeLock?.release()<!>
        }
        cleanup()
        val task = object : Runnable {
            override fun run() {
                wakeLock?.release()
            }
        }
        task.run()
    }

    override fun onPause() {
        super.onPause()
        wakeLock?.release()
    }
}

open class BaseActivity : Activity()

// An Activity through a project base class.
class ChildActivity : BaseActivity() {
    private lateinit var lock: WakeLock

    override fun onDestroy() {
        super.onDestroy()
        <!Wakelock!>lock.release()<!>
    }
}

class PlayerService : Service() {
    private lateinit var lock: WakeLock

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        super.onDestroy()
        lock.release()
    }
}
