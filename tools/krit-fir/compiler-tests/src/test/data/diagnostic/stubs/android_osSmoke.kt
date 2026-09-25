// Smoke: SDK_INT gates, Bundle, Handler/Looper, wake locks, Parcelable, AsyncTask.
package stubs

import android.content.Context
import android.os.AsyncTask
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.Message
import android.os.Parcel
import android.os.Parcelable
import android.os.PowerManager

fun sdkChecks(): Boolean {
    val sdk: Int = Build.VERSION.SDK_INT
    if (Build.VERSION.SDK_INT >= 26) <!PrintlnInProduction!>println<!>(Build.VERSION.RELEASE)
    return sdk >= Build.VERSION_CODES.O && Build.VERSION.SDK_INT < Build.VERSION_CODES.VANILLA_ICE_CREAM &&
        Build.MANUFACTURER.isNotEmpty()
}

fun bundleRoundTrip(): String {
    val bundle = Bundle()
    bundle.putString("k", "v")
    bundle.putInt("n", 1)
    bundle.putBoolean("b", true)
    val n: Int = bundle.getInt("n")
    val copy = Bundle(bundle)
    val value: String? = copy.getString("k")
    return "$value$n${copy.containsKey("b")}${copy.getString("missing", "fallback")}"
}

fun postToMain(block: Runnable) {
    val handler = Handler(Looper.getMainLooper())
    handler.post(block)
    handler.postDelayed(block, 100L)
    handler.removeCallbacks(block)
    val callbackHandler = Handler(Looper.getMainLooper()) { message: Message -> message.what == 1 }
    callbackHandler.sendMessage(Message.obtain())
    if (Looper.myLooper() == Looper.getMainLooper()) handler.removeCallbacksAndMessages(null)
}

class SmokeHandler(looper: Looper) : Handler(looper) {
    override fun handleMessage(msg: Message) {
        <!PrintlnInProduction!>println<!>(msg.obj)
    }
}

fun wakeLock(context: Context) {
    val pm = context.getSystemService(Context.POWER_SERVICE) as PowerManager
    val lock: PowerManager.WakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "smoke:tag")
    lock.acquire(10_000L)
    if (lock.isHeld) lock.release()
}

class ManualParcelable(val id: Int) : Parcelable {
    override fun describeContents(): Int = 0

    override fun writeToParcel(dest: Parcel, flags: Int) {
        dest.writeInt(id)
    }

    companion object CREATOR : Parcelable.Creator<ManualParcelable> {
        override fun createFromParcel(source: Parcel): ManualParcelable = ManualParcelable(source.readInt())

        override fun newArray(size: Int): Array<ManualParcelable?> = arrayOfNulls(size)
    }
}

@Suppress("DEPRECATION")
class SmokeTask : AsyncTask<String, Int, Boolean>() {
    @Deprecated("Deprecated in Java")
    override fun doInBackground(vararg params: String): Boolean = params.isNotEmpty()

    @Deprecated("Deprecated in Java")
    override fun onPostExecute(result: Boolean) {}
}
