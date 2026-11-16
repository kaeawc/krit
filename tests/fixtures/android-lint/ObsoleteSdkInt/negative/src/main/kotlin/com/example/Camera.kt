package com.example

import android.os.Build

class Camera {
    fun supported(): Boolean = Build.VERSION.SDK_INT >= 26
}
