package fixtures.positive.androidlint

import android.content.Context
import androidx.work.ListenableWorker
import androidx.work.PeriodicWorkRequest
import androidx.work.WorkManager
import java.util.concurrent.TimeUnit

fun schedule(context: Context, worker: Class<out ListenableWorker>) {
    val request = PeriodicWorkRequest.Builder(worker, 1L, TimeUnit.HOURS).build()
    WorkManager.getInstance(context).enqueue(request)
}
