package com.example

import android.content.Context
import androidx.work.OneTimeWorkRequest
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequest
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkRequest
import java.util.concurrent.TimeUnit

fun enqueue(context: Context) {
    val manager = WorkManager.getInstance(context)
    val periodic = PeriodicWorkRequest.Builder(SyncWorker::class.java, 1L, TimeUnit.HOURS).build()
    val periodic2: PeriodicWorkRequest = PeriodicWorkRequestBuilder<SyncWorker>(1L, TimeUnit.HOURS).build()
    val oneTime = OneTimeWorkRequestBuilder<SyncWorker>().build()
    val anyRequest: WorkRequest = oneTime
    manager.enqueue(periodic)
    manager.enqueue(periodic2)
    WorkManager.getInstance(context).enqueue(PeriodicWorkRequestBuilder<SyncWorker>(1L, TimeUnit.HOURS).build())
    manager.enqueue(oneTime)
    manager.enqueue(anyRequest)
    manager.enqueue(listOf(periodic))
    manager.enqueue(listOf(oneTime))
    manager.enqueue(listOf<WorkRequest>(oneTime))
    manager.enqueueUniquePeriodicWork("sync", androidx.work.ExistingPeriodicWorkPolicy.KEEP, periodic)
}

class FakeManager {
    fun enqueue(request: PeriodicWorkRequest) = Unit
}

fun lookalike(fake: FakeManager, periodic: PeriodicWorkRequest) {
    fake.enqueue(periodic)
}
