package fixtures.negative.androidlint

import androidx.work.ListenableWorker
import androidx.work.PeriodicWorkRequest
import java.time.Duration
import java.util.concurrent.TimeUnit

fun longEnough(worker: Class<out ListenableWorker>, minutes: Long) {
    PeriodicWorkRequest.Builder(worker, 15L, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(worker, 900L, TimeUnit.SECONDS)
    PeriodicWorkRequest.Builder(worker, Duration.ofHours(1))
    PeriodicWorkRequest.Builder(worker, minutes, TimeUnit.MINUTES)
}

class Builder(val worker: Any, val interval: Long, val unit: TimeUnit)

fun lookalike(worker: Any) {
    Builder(worker, 1L, TimeUnit.MINUTES)
}
