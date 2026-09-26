// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 31, 38, 50, 62
// Tags that reach the call indirectly and that Go also reports: a property
// initialized from another property, a safe call, and an abstract or open
// property whose only literal is an override in this file. Each tag passed is
// longer than 23 characters.
package test.longlogtag.indirect

import android.util.Log

object LogTags {
    const val TAG = "MyApplicationNetworkLayerX"
}

class Repository {
    // Repository.TAG reads LogTags.TAG, so its value is that literal.
    private val TAG = LogTags.TAG

    fun log() {
        <!LongLogTag!>Log.d(TAG, "m")<!>
    }
}

class Screen {
    val screenTag = "ScreenTagThatIsWayTooLongXX"
}

// Log's tag parameter is a platform String, so a nullable tag compiles; when a
// tag is passed it is the 27-character literal.
fun safeCall(screen: Screen?) {
    <!LongLogTag!>Log.d(screen?.screenTag, "m")<!>
}

abstract class Base {
    abstract val baseTag: String

    fun log() {
        <!LongLogTag!>Log.d(baseTag, "m")<!>
    }
}

class Impl : Base() {
    override val baseTag = "ImplementationTagThatIsTooLong"
}

interface Tagged {
    val interfaceTag: String

    fun log() {
        <!LongLogTag!>Log.i(interfaceTag, "m")<!>
    }
}

val anonymousTagged = object : Tagged {
    override val interfaceTag = "AnonymousImplementationTagX"
}

open class OpenBase {
    open val openTag: String get() = computeTag()

    fun log() {
        <!LongLogTag!>Log.w(openTag, "m")<!>
    }

    private fun computeTag() = "x"
}

class OpenImpl : OpenBase() {
    override val openTag = "OpenOverrideTagThatIsTooLong"
}

abstract class ShortBase {
    abstract val shortTag: String

    fun log() {
        Log.d(shortTag, "short override")
    }
}

class ShortImpl : ShortBase() {
    override val shortTag = "Short"
}

abstract class UnimplementedBase {
    abstract val unimplementedTag: String

    fun log() {
        Log.d(unimplementedTag, "no override in this file")
    }
}
