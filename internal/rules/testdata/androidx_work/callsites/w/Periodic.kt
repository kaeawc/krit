package com.example

import androidx.work.CoroutineWorker
import androidx.work.PeriodicWorkRequest
import androidx.work.PeriodicWorkRequestBuilder
import java.time.Duration
import java.util.concurrent.TimeUnit

abstract class SyncWorker : CoroutineWorker(TODO(), TODO())

const val FIVE = 5L

fun periodic() {
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 10L, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 10, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 899L, TimeUnit.SECONDS)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 900L, TimeUnit.SECONDS)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 15L, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 1L, TimeUnit.HOURS)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, FIVE, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(SyncWorker::class.java, Duration.ofMinutes(5))
    PeriodicWorkRequest.Builder(SyncWorker::class.java, Duration.ofSeconds(60))
    PeriodicWorkRequest.Builder(SyncWorker::class.java, Duration.ofMinutes(15))
    PeriodicWorkRequest.Builder(SyncWorker::class.java, (Duration.ofMinutes(1)))
    PeriodicWorkRequest.Builder(SyncWorker::class.java, 10L, TimeUnit.MINUTES, 5L, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(
        workerClass = SyncWorker::class.java,
        repeatIntervalTimeUnit = TimeUnit.MINUTES,
        repeatInterval = 3L,
    )
    PeriodicWorkRequestBuilder<SyncWorker>(10L, TimeUnit.MINUTES)
    PeriodicWorkRequestBuilder<SyncWorker>(Duration.ofMinutes(5))
}
