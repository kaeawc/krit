// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): Go looks a referenced tag up by name and takes the
// first same-named property in the file whose initializer is a literal. Here
// that is Short's TAG, so Go measures "Short" and misses the long TAG the call
// in Long actually reads. FIR reads the property the reference resolves to.
package test.longlogtag.recallbyname

import android.util.Log

class Short {
    companion object {
        const val TAG = "Short"
    }

    fun log() {
        Log.d(TAG, "m")
    }
}

class Long {
    companion object {
        const val TAG = "LongClassTagThatIsFarTooLong"
    }

    fun log() {
        <!LongLogTag!>Log.d(TAG, "m")<!>
    }
}
