package com.example

import android.app.Notification

class ForegroundInfo(val id: Int, val notification: Notification, val type: Int)

fun info(notification: Notification) {
    ForegroundInfo(1, notification, 1)
}
