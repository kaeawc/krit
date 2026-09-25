// Smoke: platform lint annotations on classes, functions, and constructors.
package stubs

import android.annotation.SuppressLint
import android.annotation.TargetApi
import android.os.Build

@TargetApi(Build.VERSION_CODES.O)
class OreoOnly @TargetApi(26) constructor() {
    @SuppressLint("NewApi", "MissingPermission")
    @TargetApi(Build.VERSION_CODES.P)
    fun run() {
        @SuppressLint("SetTextI18n")
        val local = "local"
        <!PrintlnInProduction!>println<!>(local)
    }
}
