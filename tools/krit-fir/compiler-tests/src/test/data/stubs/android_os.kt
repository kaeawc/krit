// Compiler-test source stubs; never packaged in the production artifact.
package android.os

open class BaseBundle {
    fun containsKey(key: String?): Boolean = TODO()

    fun isEmpty(): Boolean = TODO()

    fun keySet(): Set<String> = TODO()

    fun remove(key: String?) {
        TODO()
    }

    fun getString(key: String?): String? = TODO()

    fun getString(key: String?, defaultValue: String): String = TODO()

    fun putString(key: String?, value: String?) {
        TODO()
    }

    fun getInt(key: String?): Int = TODO()

    fun getInt(key: String?, defaultValue: Int): Int = TODO()

    fun putInt(key: String?, value: Int) {
        TODO()
    }

    fun getLong(key: String?): Long = TODO()

    fun putLong(key: String?, value: Long) {
        TODO()
    }

    fun getBoolean(key: String?): Boolean = TODO()

    fun getBoolean(key: String?, defaultValue: Boolean): Boolean = TODO()

    fun putBoolean(key: String?, value: Boolean) {
        TODO()
    }
}

class Bundle : BaseBundle, Parcelable, Cloneable {
    constructor()

    constructor(capacity: Int)

    constructor(b: Bundle?)

    @Deprecated("Deprecated in Java")
    fun <T : Parcelable> getParcelable(key: String?): T? = TODO()

    fun <T> getParcelable(key: String?, clazz: Class<T>): T? = TODO()

    fun putParcelable(key: String?, value: Parcelable?) {
        TODO()
    }

    fun getBundle(key: String?): Bundle? = TODO()

    fun putBundle(key: String?, value: Bundle?) {
        TODO()
    }

    fun getStringArrayList(key: String?): ArrayList<String>? = TODO()

    companion object {
        val EMPTY: Bundle
            get() = TODO()
    }
}

interface Parcelable {
    // Abstract in the SDK. They carry default bodies here only because the
    // kotlin-parcelize compiler plugin (which generates them for @Parcelize
    // classes) is not loaded in compiler-tests; hand-written Parcelables still
    // override them exactly as real code does.
    fun describeContents(): Int = TODO()

    fun writeToParcel(dest: Parcel, flags: Int) {
        TODO()
    }

    interface Creator<T> {
        fun createFromParcel(source: Parcel): T

        fun newArray(size: Int): Array<T?>
    }

    companion object {
        const val CONTENTS_FILE_DESCRIPTOR: Int = 1
        const val PARCELABLE_WRITE_RETURN_VALUE: Int = 1
    }
}

// App code obtains Parcels (Parcel.obtain()) and never subclasses them.
object Parcel {
    fun obtain(): Parcel = TODO()

    fun recycle() {
        TODO()
    }

    fun writeInt(value: Int) {
        TODO()
    }

    fun readInt(): Int = TODO()

    fun writeLong(value: Long) {
        TODO()
    }

    fun readLong(): Long = TODO()

    fun writeString(value: String?) {
        TODO()
    }

    fun readString(): String? = TODO()

    fun writeParcelable(p: Parcelable?, parcelableFlags: Int) {
        TODO()
    }
}

interface IBinder

open class Handler {
    @Deprecated("Deprecated in Java")
    constructor()

    constructor(looper: Looper)

    constructor(looper: Looper, callback: Callback?)

    fun post(r: Runnable): Boolean = TODO()

    fun postDelayed(r: Runnable, delayMillis: Long): Boolean = TODO()

    fun removeCallbacks(r: Runnable) {
        TODO()
    }

    fun removeCallbacksAndMessages(token: Any?) {
        TODO()
    }

    fun sendMessage(msg: Message): Boolean = TODO()

    fun sendEmptyMessage(what: Int): Boolean = TODO()

    open fun handleMessage(msg: Message) {
        TODO()
    }

    fun interface Callback {
        fun handleMessage(msg: Message): Boolean
    }
}

object Looper {
    val thread: Thread
        get() = TODO()

    fun getMainLooper(): Looper = TODO()

    fun myLooper(): Looper? = TODO()

    fun prepare() {
        TODO()
    }

    fun loop() {
        TODO()
    }
}

// App code uses the Message.obtain() pool rather than the constructor.
object Message {
    @JvmField
    var what: Int = 0

    @JvmField
    var arg1: Int = 0

    @JvmField
    var obj: Any? = null

    fun obtain(): Message = TODO()

    fun obtain(h: Handler?, what: Int): Message = TODO()

    fun sendToTarget() {
        TODO()
    }
}

object PowerManager {
    const val PARTIAL_WAKE_LOCK: Int = 1

    val isInteractive: Boolean
        get() = TODO()

    fun newWakeLock(levelAndFlags: Int, tag: String): WakeLock = TODO()

    fun isIgnoringBatteryOptimizations(packageName: String): Boolean = TODO()

    class WakeLock {
        val isHeld: Boolean
            get() = TODO()

        fun acquire() {
            TODO()
        }

        fun acquire(timeout: Long) {
            TODO()
        }

        fun release() {
            TODO()
        }
    }
}

@Deprecated("Deprecated in Java")
abstract class AsyncTask<Params, Progress, Result> {
    @Deprecated("Deprecated in Java")
    protected abstract fun doInBackground(vararg params: Params): Result

    @Deprecated("Deprecated in Java")
    protected open fun onPreExecute() {
        TODO()
    }

    @Deprecated("Deprecated in Java")
    protected open fun onPostExecute(result: Result) {
        TODO()
    }

    @Deprecated("Deprecated in Java")
    protected open fun onProgressUpdate(vararg values: Progress) {
        TODO()
    }

    @Deprecated("Deprecated in Java")
    fun execute(vararg params: Params): AsyncTask<Params, Progress, Result> = TODO()

    @Deprecated("Deprecated in Java")
    fun cancel(mayInterruptIfRunning: Boolean): Boolean = TODO()
}

object Build {
    // Static final fields read from system properties at runtime: not
    // compile-time constants, so they are @JvmField vals, not const.
    @JvmField
    val BRAND: String = TODO()

    @JvmField
    val DEVICE: String = TODO()

    @JvmField
    val FINGERPRINT: String = TODO()

    @JvmField
    val MANUFACTURER: String = TODO()

    @JvmField
    val MODEL: String = TODO()

    object VERSION {
        @JvmField
        val SDK_INT: Int = TODO()

        @JvmField
        val RELEASE: String = TODO()

        @JvmField
        val CODENAME: String = TODO()
    }

    object VERSION_CODES {
        const val BASE: Int = 1
        const val HONEYCOMB: Int = 11
        const val ICE_CREAM_SANDWICH: Int = 14
        const val JELLY_BEAN: Int = 16
        const val JELLY_BEAN_MR1: Int = 17
        const val JELLY_BEAN_MR2: Int = 18
        const val KITKAT: Int = 19
        const val LOLLIPOP: Int = 21
        const val LOLLIPOP_MR1: Int = 22
        const val M: Int = 23
        const val N: Int = 24
        const val N_MR1: Int = 25
        const val O: Int = 26
        const val O_MR1: Int = 27
        const val P: Int = 28
        const val Q: Int = 29
        const val R: Int = 30
        const val S: Int = 31
        const val S_V2: Int = 32
        const val TIRAMISU: Int = 33
        const val UPSIDE_DOWN_CAKE: Int = 34
        const val VANILLA_ICE_CREAM: Int = 35
    }
}
