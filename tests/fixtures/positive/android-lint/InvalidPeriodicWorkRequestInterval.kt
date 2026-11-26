package fixtures.positive.androidlint

import androidx.work.ListenableWorker
import androidx.work.PeriodicWorkRequest
import java.time.Duration
import java.util.concurrent.TimeUnit

fun tooShort(worker: Class<out ListenableWorker>) {
    PeriodicWorkRequest.Builder(worker, 10L, TimeUnit.MINUTES)
    PeriodicWorkRequest.Builder(worker, Duration.ofSeconds(30))
}
