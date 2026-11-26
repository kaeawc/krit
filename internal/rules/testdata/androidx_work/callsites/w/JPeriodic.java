package com.example;

import androidx.work.ListenableWorker;
import androidx.work.PeriodicWorkRequest;
import java.time.Duration;
import java.util.concurrent.TimeUnit;

class JPeriodic {
    static final long TEN = 10L;

    void periodic(Class<? extends ListenableWorker> worker) {
        new PeriodicWorkRequest.Builder(worker, 10L, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(worker, 10, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(worker, TEN, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(worker, 20L, TimeUnit.MINUTES);
        new PeriodicWorkRequest.Builder(worker, Duration.ofMinutes(5));
        new PeriodicWorkRequest.Builder(worker, Duration.ofHours(1));
        new PeriodicWorkRequest.Builder(worker, 1L, TimeUnit.MINUTES, 1L, TimeUnit.MINUTES);
    }
}
