package com.example

import android.os.Build

class Camera {
    // The module's minSdk is 24, so the check is always true.
    fun supported(): Boolean = Build.VERSION.SDK_INT >= 21
}
