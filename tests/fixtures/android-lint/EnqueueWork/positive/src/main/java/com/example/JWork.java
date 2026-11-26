package com.example;

import android.content.Context;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkContinuation;
import androidx.work.WorkManager;

class JWork {
    void schedule(Context context, OneTimeWorkRequest a) {
        WorkContinuation c = WorkManager.getInstance(context).beginWith(a);
        c.getWorkInfos();
    }
}
