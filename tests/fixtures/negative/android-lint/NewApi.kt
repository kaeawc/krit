package test

import android.os.Build

fun createChannel() {
    if (Build.VERSION.SDK_INT >= 26) {
        setupNotification()
    }
}
