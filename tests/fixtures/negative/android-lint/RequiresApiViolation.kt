package test

import android.os.Build
import androidx.annotation.RequiresApi

@RequiresApi(26)
fun newApiHelper() {
}

class Caller {
    fun guardedBySdkInt() {
        if (Build.VERSION.SDK_INT >= 26) {
            newApiHelper()
        }
    }

    @RequiresApi(26)
    fun guardedByAnnotation() {
        newApiHelper()
    }

    fun plainCall() {
        // No @RequiresApi on the target — must not fire.
        regularHelper()
    }
}

fun regularHelper() {
}
