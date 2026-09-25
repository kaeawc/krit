// Compiler-test source stubs; never packaged in the production artifact.
package android.content

import android.content.pm.PackageManager
import android.content.res.Resources
import android.content.res.TypedArray
import android.database.Cursor
import android.net.Uri
import android.os.Bundle
import android.os.Looper
import android.os.Parcelable
import android.util.AttributeSet
import java.io.InputStream

abstract class Context {
    abstract val resources: Resources

    abstract val theme: Resources.Theme

    abstract val packageName: String

    abstract val packageManager: PackageManager

    abstract val contentResolver: ContentResolver

    abstract val applicationContext: Context

    abstract val mainLooper: Looper

    fun getString(resId: Int): String = TODO()

    fun getString(resId: Int, vararg formatArgs: Any?): String = TODO()

    fun getText(resId: Int): CharSequence = TODO()

    fun getColor(id: Int): Int = TODO()

    fun obtainStyledAttributes(attrs: IntArray): TypedArray = TODO()

    fun obtainStyledAttributes(set: AttributeSet?, attrs: IntArray): TypedArray = TODO()

    abstract fun getSharedPreferences(name: String, mode: Int): SharedPreferences

    abstract fun startActivity(intent: Intent)

    abstract fun startService(service: Intent): ComponentName?

    abstract fun sendBroadcast(intent: Intent)

    abstract fun registerReceiver(receiver: BroadcastReceiver?, filter: IntentFilter): Intent?

    abstract fun registerReceiver(receiver: BroadcastReceiver?, filter: IntentFilter, flags: Int): Intent?

    abstract fun unregisterReceiver(receiver: BroadcastReceiver)

    abstract fun enforceCallingPermission(permission: String, message: String?)

    abstract fun checkCallingOrSelfPermission(permission: String): Int

    abstract fun checkSelfPermission(permission: String): Int

    abstract fun getSystemService(name: String): Any?

    fun <T> getSystemService(serviceClass: Class<T>): T? = TODO()

    companion object {
        const val MODE_PRIVATE: Int = 0
        const val RECEIVER_EXPORTED: Int = 2
        const val RECEIVER_NOT_EXPORTED: Int = 4
        const val ACTIVITY_SERVICE: String = "activity"
        const val BLUETOOTH_SERVICE: String = "bluetooth"
        const val CAMERA_SERVICE: String = "camera"
        const val CONNECTIVITY_SERVICE: String = "connectivity"
        const val LAYOUT_INFLATER_SERVICE: String = "layout_inflater"
        const val LOCATION_SERVICE: String = "location"
        const val NOTIFICATION_SERVICE: String = "notification"
        const val POWER_SERVICE: String = "power"
    }
}

open class ContextWrapper(base: Context?) : Context() {
    val baseContext: Context
        get() = TODO()

    override val resources: Resources
        get() = TODO()

    override val theme: Resources.Theme
        get() = TODO()

    override val packageName: String
        get() = TODO()

    override val packageManager: PackageManager
        get() = TODO()

    override val contentResolver: ContentResolver
        get() = TODO()

    override val applicationContext: Context
        get() = TODO()

    override val mainLooper: Looper
        get() = TODO()

    protected open fun attachBaseContext(base: Context) {
        TODO()
    }

    override fun getSharedPreferences(name: String, mode: Int): SharedPreferences = TODO()

    override fun startActivity(intent: Intent) {
        TODO()
    }

    override fun startService(service: Intent): ComponentName? = TODO()

    override fun sendBroadcast(intent: Intent) {
        TODO()
    }

    override fun registerReceiver(receiver: BroadcastReceiver?, filter: IntentFilter): Intent? = TODO()

    override fun registerReceiver(receiver: BroadcastReceiver?, filter: IntentFilter, flags: Int): Intent? = TODO()

    override fun unregisterReceiver(receiver: BroadcastReceiver) {
        TODO()
    }

    override fun enforceCallingPermission(permission: String, message: String?) {
        TODO()
    }

    override fun checkCallingOrSelfPermission(permission: String): Int = TODO()

    override fun checkSelfPermission(permission: String): Int = TODO()

    override fun getSystemService(name: String): Any? = TODO()
}

open class Intent : Parcelable, Cloneable {
    constructor()

    constructor(action: String?)

    constructor(action: String?, uri: Uri?)

    constructor(packageContext: Context, cls: Class<*>)

    constructor(o: Intent)

    open var action: String?
        get() = TODO()
        set(value) = TODO()

    open var data: Uri?
        get() = TODO()
        set(value) = TODO()

    open val extras: Bundle?
        get() = TODO()

    open fun getStringExtra(name: String): String? = TODO()

    open fun getIntExtra(name: String, defaultValue: Int): Int = TODO()

    open fun getBooleanExtra(name: String, defaultValue: Boolean): Boolean = TODO()

    open fun hasExtra(name: String): Boolean = TODO()

    open fun putExtra(name: String, value: String?): Intent = TODO()

    open fun putExtra(name: String, value: Int): Intent = TODO()

    open fun putExtra(name: String, value: Boolean): Intent = TODO()

    open fun putExtra(name: String, value: Parcelable?): Intent = TODO()

    open fun addFlags(flags: Int): Intent = TODO()

    open fun setPackage(packageName: String?): Intent = TODO()

    open fun setClass(packageContext: Context, cls: Class<*>): Intent = TODO()

    companion object {
        const val ACTION_BOOT_COMPLETED: String = "android.intent.action.BOOT_COMPLETED"
        const val ACTION_MAIN: String = "android.intent.action.MAIN"
        const val ACTION_SEND: String = "android.intent.action.SEND"
        const val ACTION_VIEW: String = "android.intent.action.VIEW"
        const val CATEGORY_LAUNCHER: String = "android.intent.category.LAUNCHER"
        const val EXTRA_TEXT: String = "android.intent.extra.TEXT"
        const val FLAG_ACTIVITY_CLEAR_TOP: Int = 67108864
        const val FLAG_ACTIVITY_NEW_TASK: Int = 268435456

        fun createChooser(target: Intent, title: CharSequence?): Intent = TODO()
    }
}

open class IntentFilter : Parcelable {
    constructor()

    constructor(action: String)

    fun addAction(action: String) {
        TODO()
    }

    fun addCategory(category: String) {
        TODO()
    }
}

class ComponentName(pkg: String, cls: String) {
    constructor(pkg: Context, cls: Class<*>) : this("", "")

    val packageName: String
        get() = TODO()

    val className: String
        get() = TODO()
}

class ContentValues {
    fun put(key: String, value: String?) {
        TODO()
    }

    fun put(key: String, value: Int?) {
        TODO()
    }

    fun put(key: String, value: Long?) {
        TODO()
    }

    fun put(key: String, value: Boolean?) {
        TODO()
    }

    fun putNull(key: String) {
        TODO()
    }
}

interface SharedPreferences {
    fun getString(key: String, defValue: String?): String?

    fun getStringSet(key: String, defValues: Set<String>?): Set<String>?

    fun getInt(key: String, defValue: Int): Int

    fun getLong(key: String, defValue: Long): Long

    fun getFloat(key: String, defValue: Float): Float

    fun getBoolean(key: String, defValue: Boolean): Boolean

    fun contains(key: String): Boolean

    fun edit(): Editor

    fun registerOnSharedPreferenceChangeListener(listener: OnSharedPreferenceChangeListener)

    fun unregisterOnSharedPreferenceChangeListener(listener: OnSharedPreferenceChangeListener)

    interface Editor {
        fun putString(key: String, value: String?): Editor

        fun putStringSet(key: String, values: Set<String>?): Editor

        fun putInt(key: String, value: Int): Editor

        fun putLong(key: String, value: Long): Editor

        fun putFloat(key: String, value: Float): Editor

        fun putBoolean(key: String, value: Boolean): Editor

        fun remove(key: String): Editor

        fun clear(): Editor

        fun commit(): Boolean

        fun apply()
    }

    fun interface OnSharedPreferenceChangeListener {
        fun onSharedPreferenceChanged(sharedPreferences: SharedPreferences, key: String?)
    }
}

abstract class BroadcastReceiver {
    // Unannotated Java parameters; the Android Studio template overrides with
    // non-null Context and Intent.
    abstract fun onReceive(context: Context, intent: Intent)

    fun goAsync(): PendingResult = TODO()

    val isOrderedBroadcast: Boolean
        get() = TODO()

    class PendingResult {
        fun finish() {
            TODO()
        }
    }
}

abstract class ContentProvider {
    val context: Context?
        get() = TODO()

    fun requireContext(): Context = TODO()

    abstract fun onCreate(): Boolean

    abstract fun query(
        uri: Uri,
        projection: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        sortOrder: String?,
    ): Cursor?

    abstract fun getType(uri: Uri): String?

    abstract fun insert(uri: Uri, values: ContentValues?): Uri?

    abstract fun delete(uri: Uri, selection: String?, selectionArgs: Array<String>?): Int

    abstract fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<String>?): Int
}

abstract class ContentResolver(context: Context?) {
    fun query(
        uri: Uri,
        projection: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        sortOrder: String?,
    ): Cursor? = TODO()

    fun insert(url: Uri, values: ContentValues?): Uri? = TODO()

    fun update(uri: Uri, values: ContentValues?, where: String?, selectionArgs: Array<String>?): Int = TODO()

    fun delete(url: Uri, where: String?, selectionArgs: Array<String>?): Int = TODO()

    fun openInputStream(uri: Uri): InputStream? = TODO()

    fun notifyChange(uri: Uri, observer: Any?) {
        TODO()
    }
}

interface DialogInterface {
    fun cancel()

    fun dismiss()

    fun interface OnClickListener {
        fun onClick(dialog: DialogInterface, which: Int)
    }

    companion object {
        const val BUTTON_POSITIVE: Int = -1
        const val BUTTON_NEGATIVE: Int = -2
    }
}

@Deprecated("Deprecated in Java")
open class CursorLoader(context: Context) {
    constructor(
        context: Context,
        uri: Uri,
        projection: Array<String>?,
        selection: String?,
        selectionArgs: Array<String>?,
        sortOrder: String?,
    ) : this(context)

    open fun loadInBackground(): Cursor? = TODO()
}
