package com.example;

import android.content.Context;
import androidx.work.ListenableWorker;
import androidx.work.OneTimeWorkRequest;
import androidx.work.PeriodicWorkRequest;
import androidx.work.WorkManager;
import java.util.Collections;
import java.util.concurrent.TimeUnit;

class JEnqueue {
    void enqueue(Context context, Class<? extends ListenableWorker> worker) {
        PeriodicWorkRequest periodic = new PeriodicWorkRequest.Builder(worker, 1L, TimeUnit.HOURS).build();
        OneTimeWorkRequest oneTime = new OneTimeWorkRequest.Builder(worker).build();
        WorkManager.getInstance(context).enqueue(periodic);
        WorkManager.getInstance(context).enqueue(oneTime);
        WorkManager.getInstance(context).enqueue(Collections.singletonList(periodic));
    }

    static class Fake {
        void enqueue(PeriodicWorkRequest request) {}
    }

    void lookalike(Fake fake, PeriodicWorkRequest periodic) {
        fake.enqueue(periodic);
    }
}
