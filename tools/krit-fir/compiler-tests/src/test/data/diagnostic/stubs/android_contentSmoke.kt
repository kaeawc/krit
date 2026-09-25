// Smoke: receivers, providers, wrappers, intents, and shared preferences.
package stubs

import android.content.BroadcastReceiver
import android.content.ContentProvider
import android.content.ContentValues
import android.content.Context
import android.content.ContextWrapper
import android.content.DialogInterface
import android.content.Intent
import android.content.IntentFilter
import android.content.SharedPreferences
import android.database.Cursor
import android.net.Uri

class SmokeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val extra: String? = intent.getStringExtra("key")
        println(context.getString(android.R.string.ok))
        println(context.getString(android.R.string.ok, extra))
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) goAsync().finish()
    }
}

class SmokeProvider : ContentProvider() {
    override fun onCreate(): Boolean = context != null

    override fun query(
        uri: Uri,
        projection: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        sortOrder: String?,
    ): Cursor? = null

    override fun getType(uri: Uri): String? = null

    override fun insert(uri: Uri, values: ContentValues?): Uri? = null

    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int = 0

    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<String>?): Int = 0
}

class SmokeContextWrapper(base: Context) : ContextWrapper(base) {
    override fun attachBaseContext(base: Context) {
        super.attachBaseContext(base)
    }
}

fun registerAndSave(context: Context, receiver: BroadcastReceiver, dialog: DialogInterface) {
    val filter = IntentFilter(Intent.ACTION_BOOT_COMPLETED)
    filter.addAction("smoke.ACTION")
    context.registerReceiver(receiver, filter)
    context.registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED)
    context.sendBroadcast(Intent("smoke.ACTION").putExtra("key", "value").setPackage(context.packageName))
    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://example.com")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    context.unregisterReceiver(receiver)
    val prefs: SharedPreferences = context.getSharedPreferences("smoke", Context.MODE_PRIVATE)
    prefs.edit().putString("key", "value").apply()
    val committed: Boolean = prefs.edit().remove("key").commit()
    val stored: String? = prefs.getString("key", null)
    prefs.registerOnSharedPreferenceChangeListener { _, key -> println(key) }
    context.enforceCallingPermission(android.Manifest.permission.CAMERA, "camera")
    val granted: Int = context.checkCallingOrSelfPermission(android.Manifest.permission.CAMERA)
    val attrs = context.obtainStyledAttributes(intArrayOf(1))
    attrs.recycle()
    println(context.resources.getString(android.R.string.cancel))
    context.contentResolver.query(Uri.parse("content://smoke"), null, null, null, null)?.close()
    context.applicationContext.getString(android.R.string.ok)
    dialog.dismiss()
    println("$committed $stored $granted")
}
