package com.example

import android.app.Notification
import android.content.pm.ServiceInfo
import androidx.work.ForegroundInfo

fun info(notification: Notification) {
    ForegroundInfo(1, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
    ForegroundInfo(2, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION or ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
    ForegroundInfo(3, notification, 16)
    ForegroundInfo(4, notification)
    ForegroundInfo(5, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA)
    ForegroundInfo(6, notification, 0)
}
