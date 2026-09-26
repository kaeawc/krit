// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 17, 20, 24, 25, 26, 27, 28, 29, 30, 31, 33, 45, 52, 58, 64, 120, 140, 143, 146, 149, 152, 153, 157, 163, 173, 174, 176, 179, 185, 190, 211, 214
package test.logconditional

import android.util.Log

// The app's generated BuildConfig; the golden declares its own.
object BuildConfig {
    val DEBUG: Boolean = true
}

private const val TAG = "Tag"

private val atTopLevel = <!LogConditional!>Log.i(TAG, "top-level initializer")<!>

class Levels {
    val inMember = <!LogConditional!>Log.w(TAG, "member initializer")<!>

    init {
        <!LogConditional!>Log.e(TAG, "init block")<!>
    }

    fun levels(error: Throwable) {
        <!LogConditional!>Log.v(TAG, "verbose")<!>
        <!LogConditional!>Log.d(TAG, "debug")<!>
        <!LogConditional!>Log.i(TAG, "info")<!>
        <!LogConditional!>Log.w(TAG, "warn")<!>
        <!LogConditional!>Log.w(TAG, error)<!>
        <!LogConditional!>Log.e(TAG, "error", error)<!>
        <!LogConditional!>android.util.Log.d(TAG, "qualified")<!>
        <!LogConditional!>Log<!>
            .d(TAG, "split over lines")
        val count = <!LogConditional!>Log.d(TAG, "value used")<!> + 1
    }

    fun notLevels(error: Throwable) {
        Log.wtf(TAG, "wtf is not a level call")
        Log.println(Log.INFO, TAG, "println is not a level call")
        Log.isLoggable(TAG, Log.DEBUG)
        Log.getStackTraceString(error)
    }

    companion object {
        fun inCompanion() {
            <!LogConditional!>Log.d(TAG, "companion")<!>
        }
    }
}

object Singleton {
    fun inObject() {
        <!LogConditional!>Log.d(TAG, "object")<!>
    }
}

interface WithDefault {
    fun inInterface() {
        <!LogConditional!>Log.d(TAG, "interface default")<!>
    }
}

fun anonymousObject(): Runnable = object : Runnable {
    override fun run() {
        <!LogConditional!>Log.d(TAG, "anonymous object member")<!>
    }
}

fun isLoggableGuards(verbose: Boolean) {
    if (Log.isLoggable(TAG, Log.DEBUG)) Log.d(TAG, "guarded")
    if (Log.isLoggable(TAG, Log.DEBUG)) {
        Log.d(TAG, "guarded block")
        if (verbose) {
            Log.v(TAG, "nested if inside the guard")
        }
    }
    if (verbose && Log.isLoggable(TAG, Log.VERBOSE)) {
        Log.v(TAG, "isLoggable inside a compound condition")
    }
    if (android.util.Log.isLoggable(TAG, Log.DEBUG)) {
        Log.d(TAG, "qualified isLoggable")
    }
    val message = if (Log.isLoggable(TAG, Log.INFO)) Log.i(TAG, "if expression") else 0
}

// Go treats an `if` as a guard when an isLoggable call appears anywhere in it,
// not only in its condition: the else branch, and a sibling isLoggable in the
// same branch, count too. FIR matches.
fun isLoggableAnywhereInTheIf(verbose: Boolean) {
    if (Log.isLoggable(TAG, Log.DEBUG)) {
        Log.d(TAG, "then")
    } else {
        Log.w(TAG, "else branch of an isLoggable if")
    }
    if (verbose) {
        Log.d(TAG, "sibling of an isLoggable check")
        val loggable = Log.isLoggable(TAG, Log.VERBOSE)
    }
}

fun buildConfigGuards(verbose: Boolean) {
    if (BuildConfig.DEBUG) Log.d(TAG, "guarded")
    if ((BuildConfig.DEBUG)) {
        Log.d(TAG, "parenthesized guard")
    }
    if (test.logconditional.BuildConfig.DEBUG) {
        Log.d(TAG, "qualified guard")
    }
    if (BuildConfig.DEBUG) {
        Log.d(TAG, "then")
    } else {
        Log.d(TAG, "else branch of a BuildConfig.DEBUG if")
    }
    if (BuildConfig.DEBUG) {
        if (verbose) Log.v(TAG, "nested inside the guard")
    }
}

fun elseIfChains(verbose: Boolean, quiet: Boolean) {
    if (verbose) {
        <!LogConditional!>Log.d(TAG, "then of the first if")<!>
    } else if (BuildConfig.DEBUG) {
        Log.d(TAG, "then of a BuildConfig.DEBUG else-if")
    } else {
        Log.d(TAG, "else after a BuildConfig.DEBUG else-if")
    }
    if (BuildConfig.DEBUG) {
        Log.d(TAG, "guarded")
    } else if (quiet) {
        Log.d(TAG, "else-if after a BuildConfig.DEBUG if")
    }
    if (verbose) {
        Log.d(TAG, "an isLoggable later in the chain guards the whole chain")
    } else if (Log.isLoggable(TAG, Log.DEBUG)) {
        Log.d(TAG, "guarded")
    }
}

fun notGuards(verbose: Boolean, level: Int) {
    if (verbose) {
        <!LogConditional!>Log.d(TAG, "plain condition")<!>
    }
    if (!BuildConfig.DEBUG) {
        <!LogConditional!>Log.d(TAG, "negated")<!>
    }
    if (BuildConfig.DEBUG && verbose) {
        <!LogConditional!>Log.d(TAG, "combined condition")<!>
    }
    if (BuildConfig.DEBUG == true) {
        <!LogConditional!>Log.d(TAG, "comparison")<!>
    }
    when {
        BuildConfig.DEBUG -> <!LogConditional!>Log.d(TAG, "when is not a guard")<!>
        Log.isLoggable(TAG, level) -> <!LogConditional!>Log.d(TAG, "when is not a guard")<!>
    }
    val enabled = BuildConfig.DEBUG
    if (enabled) {
        <!LogConditional!>Log.d(TAG, "local copy of the flag")<!>
    }
    // Go needs the condition text `BuildConfig.DEBUG`; an implicit receiver
    // spells it `DEBUG`, so Go reports this, and FIR matches.
    with(BuildConfig) {
        if (DEBUG) {
            <!LogConditional!>Log.d(TAG, "DEBUG through an implicit receiver")<!>
        }
    }
}

// Go stops at the nearest function, lambda, anonymous function, or named class
// or object, so a guard outside it does not cover the call. An anonymous
// object is passed through, as Go passes `object_literal`.
fun scopeBoundaries(items: List<String>) {
    if (BuildConfig.DEBUG) {
        items.forEach { <!LogConditional!>Log.d(TAG, it)<!> }
        val callback = fun() { <!LogConditional!>Log.d(TAG, "anonymous function")<!> }
        fun local() {
            <!LogConditional!>Log.d(TAG, "local function")<!>
        }
        class Local {
            val inLocalClass = <!LogConditional!>Log.d(TAG, "local class")<!>
        }
        val holder = object {
            val inAnonymousObject = Log.d(TAG, "anonymous object initializer")

            fun member() {
                <!LogConditional!>Log.d(TAG, "anonymous object member")<!>
            }
        }
    }
    if (Log.isLoggable(TAG, Log.DEBUG)) {
        items.forEach { <!LogConditional!>Log.d(TAG, it)<!> }
    }
    if (items.any { Log.isLoggable(it, Log.DEBUG) }) {
        Log.d(TAG, "isLoggable inside a lambda in the condition")
    }
}

class Flags {
    val DEBUG: Boolean = false
    val VERBOSE: Boolean = false
}

// Go matches the condition text `BuildConfig.DEBUG`, so a DEBUG flag read
// through a receiver spelled BuildConfig guards the call whatever its type.
// FIR matches; another flag, or DEBUG through another receiver, does not.
fun buildConfigSpelledReceiver(flags: Flags) {
    val BuildConfig = flags
    if (BuildConfig.DEBUG) {
        Log.d(TAG, "DEBUG through a receiver spelled BuildConfig")
    }
    if (BuildConfig.VERBOSE) {
        <!LogConditional!>Log.d(TAG, "another flag")<!>
    }
    if (flags.DEBUG) {
        <!LogConditional!>Log.d(TAG, "DEBUG through another receiver")<!>
    }
}
