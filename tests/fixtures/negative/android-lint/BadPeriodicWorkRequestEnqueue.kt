package fixtures.negative.androidlint

import android.content.Context
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ListenableWorker
import androidx.work.OneTimeWorkRequest
import androidx.work.PeriodicWorkRequest
import androidx.work.WorkManager
import java.util.concurrent.TimeUnit

fun schedule(context: Context, worker: Class<out ListenableWorker>) {
    val manager = WorkManager.getInstance(context)
    val request = PeriodicWorkRequest.Builder(worker, 1L, TimeUnit.HOURS).build()
    manager.enqueueUniquePeriodicWork("sync", ExistingPeriodicWorkPolicy.KEEP, request)
    manager.enqueue(OneTimeWorkRequest.Builder(worker).build())
}

class Queue {
    fun enqueue(request: PeriodicWorkRequest) = Unit
}

fun lookalike(queue: Queue, request: PeriodicWorkRequest) {
    queue.enqueue(request)
}
