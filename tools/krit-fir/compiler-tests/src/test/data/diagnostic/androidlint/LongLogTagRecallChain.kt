// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each tag below reaches a literal longer than 23
// characters through a property of a different name, so the call logs with a
// tag over the limit. Go looks a referenced tag up by its own name and needs
// that property's initializer to be a literal, so it misses every chain here.
// It also takes only the first same-named literal in the file, so it measures
// ShortFirst's "Short" and misses LongSecond's override.
package test.longlogtag.recallchain

import android.util.Log

object LogTags {
    const val NETWORK = "MyApplicationNetworkLayerX"
}

private val DERIVED = LogTags.NETWORK
private val TWICE_DERIVED = DERIVED

class Network {
    private val TAG = LogTags.NETWORK

    fun log() {
        <!LongLogTag!>Log.d(TAG, "member from object const")<!>
        <!LongLogTag!>Log.d(TWICE_DERIVED, "two hops")<!>
        val local = DERIVED
        <!LongLogTag!>Log.d(local, "local from top-level")<!>
    }
}

abstract class Multi {
    abstract val multiTag: String

    fun log() {
        <!LongLogTag!>Log.d(multiTag, "second override is long")<!>
    }
}

class ShortFirst : Multi() {
    override val multiTag = "Short"
}

class LongSecond : Multi() {
    override val multiTag = "SecondOverrideTagThatIsTooLong"
}
