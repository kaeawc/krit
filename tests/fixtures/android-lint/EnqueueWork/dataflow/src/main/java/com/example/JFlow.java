package com.example;

import android.content.Context;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkContinuation;
import androidx.work.WorkManager;

class JFlow {
    static Context appContext;

    static {
        WorkManager.getInstance(appContext).beginWith(OneTimeWorkRequest.from(androidx.work.ListenableWorker.class));
    }

    void alias(Context context, OneTimeWorkRequest a) {
        WorkContinuation c = WorkManager.getInstance(context).beginWith(a);
        WorkContinuation d = c;
        d.enqueue();
    }

    void ternary(Context context, boolean x, OneTimeWorkRequest a, OneTimeWorkRequest b) {
        WorkManager wm = WorkManager.getInstance(context);
        WorkContinuation c = x ? wm.beginWith(a) : wm.beginWith(b);
        c.enqueue();
    }
}
