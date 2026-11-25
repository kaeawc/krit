package com.example.synth

import android.app.Notification
import android.content.pm.ServiceInfo
import androidx.work.ForegroundInfo

fun uploadInfo(notification: Notification) =
    ForegroundInfo(42, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
