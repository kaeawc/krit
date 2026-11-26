package com.example;

import android.app.Notification;
import android.content.pm.ServiceInfo;
import androidx.work.ForegroundInfo;

class JFg {
    static final int TYPE = ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK;

    void info(Notification notification) {
        new ForegroundInfo(1, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION);
        new ForegroundInfo(2, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC | ServiceInfo.FOREGROUND_SERVICE_TYPE_PHONE_CALL);
        new ForegroundInfo(3, notification, TYPE);
    }
}
