package p

import android.app.Activity
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.IBinder

class TrampolineReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val i = Intent(context, MainActivity::class.java)
        context.startActivity(i)
    }
}

class QuietReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        context.sendBroadcast(intent)
    }
}

class TrampolineService : Service() {
    override fun onStartCommand(intent: Intent, flags: Int, startId: Int): Int {
        startActivity(Intent(this, MainActivity::class.java))
        return 0
    }
    override fun onBind(intent: Intent): IBinder? = null
}


