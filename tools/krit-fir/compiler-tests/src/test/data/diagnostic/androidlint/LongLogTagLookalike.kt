// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 28, 29
// Divergence (precision): these `Log` classes are project and local
// declarations, not android.util.Log, and have no 23-character tag limit, so
// FIR does not report them. Go matches the receiver name `Log` and drops
// these calls only when the Kotlin oracle resolves them elsewhere; without it
// (as here) it reports each call on a receiver spelled `Log` with a long tag.
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
