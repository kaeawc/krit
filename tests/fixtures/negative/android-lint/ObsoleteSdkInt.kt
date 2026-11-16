package com.example

import android.os.Build
import androidx.annotation.RequiresApi

class Camera {
    @RequiresApi(26)
    fun open() {}

    fun supported(): Boolean = Build.VERSION.SDK_INT >= 26
}
