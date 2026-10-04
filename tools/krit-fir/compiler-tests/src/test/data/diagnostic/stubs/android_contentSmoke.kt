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
import java.io.FileOutputStream

class SmokeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val extra: String? = intent.getStringExtra("key")
        <!PrintlnInProduction!>println<!>(context.getString(android.R.string.ok))
        <!PrintlnInProduction!>println<!>(context.getString(android.R.string.ok, extra))
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

    override fun openFileOutput(name: String, mode: Int): FileOutputStream = super.openFileOutput(name, mode)
}

fun copyPrivateFile(context: Context) {
    val bytes = context.openFileInput("smoke.bin").use { it.readBytes() }
    context.openFileOutput("smoke.copy", Context.MODE_PRIVATE).use { it.write(bytes) }
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
    prefs.registerOnSharedPreferenceChangeListener { _, key -> <!PrintlnInProduction!>println<!>(key) }
    context.enforceCallingPermission(android.Manifest.permission.CAMERA, "camera")
    val granted: Int = context.checkCallingOrSelfPermission(android.Manifest.permission.CAMERA)
    val attrs = context.obtainStyledAttributes(intArrayOf(1))
    attrs.recycle()
    <!PrintlnInProduction!>println<!>(context.resources.getString(android.R.string.cancel))
    context.contentResolver.query(Uri.parse("content://smoke"), null, null, null, null)?.close()
    context.applicationContext.getString(android.R.string.ok)
    dialog.dismiss()
    val shared = Uri.parse("content://smoke/items/1")
    val share = Intent(Intent.ACTION_SEND).setData(shared)
        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
    context.startActivity(Intent.createChooser(share, "Share"))
    context.revokeUriPermission(shared, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    <!PrintlnInProduction!>println<!>("$committed $stored $granted")
}
