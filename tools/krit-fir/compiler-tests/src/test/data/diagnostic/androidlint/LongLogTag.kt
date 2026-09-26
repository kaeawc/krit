// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 28, 29, 30, 31, 35, 36, 37, 38, 39, 41, 42, 43, 44, 46
// Positives and negatives that match Go: an android.util.Log level call whose
// tag, a literal or a property initialized with a literal, is longer than 23
// characters. The finding sits on the call's first line.
package test.longlogtag

import android.util.Log

private const val FILE_TAG = "FileLevelTagThatIsFarTooLong"

object Tags {
    const val SHARED = "SharedTagNameThatIsFarTooLong"
}

class Screen {
    companion object {
        private const val TAG = "ScreenWithAVeryLongTagName"
        private const val SHORT = "Screen"
    }

    val memberTag = "MemberTagThatIsAlsoTooLongForLog"
    var mutableTag = "MutableTagThatIsAlsoTooLongX"

    fun levels(error: Throwable) {
        <!LongLogTag!>Log.v("VerboseTagThatIsWayTooLongX", "m")<!>
        <!LongLogTag!>Log.d("DebugTagThatIsWayTooLongXXX", "m")<!>
        <!LongLogTag!>Log.i("InfoTagThatIsWayTooLongXXXX", "m", error)<!>
        <!LongLogTag!>Log.w("WarnTagThatIsWayTooLongXXXX", error)<!>
        <!LongLogTag!>Log.e("ErrorTagThatIsWayTooLongXXX", "m", error)<!>
        <!LongLogTag!>Log.wtf("WtfTagThatIsWayTooLongXXXXX", "m")<!>
    }

    fun references() {
        <!LongLogTag!>Log.d(TAG, "companion const")<!>
        <!LongLogTag!>Log.d(Tags.SHARED, "object const")<!>
        <!LongLogTag!>Log.d(FILE_TAG, "top-level const")<!>
        <!LongLogTag!>Log.d(memberTag, "member val")<!>
        <!LongLogTag!>Log.d(this.mutableTag, "member var")<!>
        val localTag = "LocalTagThatIsDefinitelyTooLong"
        <!LongLogTag!>Log.d(localTag, "local val")<!>
        <!LongLogTag!>Log.d(("ParenthesizedTagThatIsTooLong"), "parenthesized")<!>
        <!LongLogTag!>Log.d("""RawStringTagThatIsTooLongXX""", "raw string")<!>
        <!LongLogTag!>Log<!>
            .d("MultiLineCallTagThatIsTooLong", "reported on the receiver line")
        listOf(1).forEach { <!LongLogTag!>Log.d("InsideLambdaTagThatIsTooLong", "m")<!> }
    }

    fun negatives(tag: String, count: Int) {
        Log.d("ExactlyTwentyThreeChars", "23 characters is allowed")
        Log.d(SHORT, "short const")
        Log.d("Screen", "short literal")
        Log.d(tag, "parameter")
        Log.d("InterpolatedTagThatIsTooLong$count", "template")
        Log.d("TagBuiltFromParts" + "ThatIsTooLong", "concatenation")
        Log.println(Log.DEBUG, "PrintlnTagThatIsWayTooLongX", "not a level call")
        Log.isLoggable("IsLoggableTagThatIsWayTooLong", Log.DEBUG)
    }
}
