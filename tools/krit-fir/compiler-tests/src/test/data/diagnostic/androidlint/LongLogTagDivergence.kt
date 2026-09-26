// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 32, 37, 47, 53, 55, 57
// Divergences (precision) where the tag Go measures is not the tag the call
// passes. FIR reads the property the reference resolves to and counts the
// runtime value's characters.
package test.longlogtag.divergence

import android.util.Log

class First {
    companion object {
        const val TAG = "FirstClassTagThatIsFarTooLong"
    }
}

class Second {
    companion object {
        // Go looks TAG up by name and takes First's literal, the first TAG in
        // the file; this class's TAG is short.
        const val TAG = "Second"
    }

    fun log() {
        Log.d(TAG, "m")
    }
}

class Shadowing {
    fun log() {
        // The local TAG shadows First.TAG; Go takes First's literal.
        val TAG = "Local"
        Log.d(TAG, "m")
    }

    fun parameter(TAG: String) {
        // A parameter has no initializer; Go takes First's literal.
        Log.d(TAG, "m")
    }
}

class Getter {
    // The getter returns a short tag; Go reads the initializer.
    val getterTag: String = "GetterBackedTagThatIsTooLong"
        get() = field.take(10)

    fun log() {
        Log.d(getterTag, "m")
    }
}

fun characters() {
    // 20 characters of non-ASCII text: 40 UTF-8 bytes, which Go counts.
    Log.d("ÄÖÜäöüÄÖÜäöüÄÖÜäöüÄÖ", "m")
    // 21 characters at runtime; Go counts the 28-character escaped spelling.
    Log.d("Tab\tSeparated\tTagéNam", "m")
    // Both count this one: 25 characters at runtime.
    <!LongLogTag!>Log.d("TagWithEscapedTab\tTooLong", "m")<!>
}
