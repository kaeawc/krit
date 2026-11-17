// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// These `Log` classes are project and local declarations, not
// android.util.Log, and have no 23-character tag limit, so neither engine
// reports them. Without the oracle, Go reads the imports: the file does not
// import android.util.Log and declares a `Log` of its own.
package test.longlogtag.lookalike

object Log {
    fun d(tag: String, message: String) {}
    fun e(tag: String, message: String) {}
}

class Holder {
    class Log {
        companion object {
            fun w(tag: String, message: String) {}
        }
    }

    fun nested() {
        Log.w("NestedLookalikeTagThatIsTooLong", "m")
    }
}

fun projectObject() {
    Log.d("ProjectLoggerTagThatIsTooLongX", "m")
    Log.e("ProjectLoggerTagThatIsTooLongX", "m")
}

fun localClass() {
    class Log {
        fun i(tag: String, message: String) {}
    }
    Log().i("LocalLookalikeTagThatIsTooLong", "m")
    val logger = object {
        fun v(tag: String, message: String) {}
    }
    logger.v("AnonymousObjectTagThatIsTooLong", "m")
}
