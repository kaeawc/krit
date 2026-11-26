package com.example

import android.content.Context
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkManager

fun schedule(context: Context, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    val manager = WorkManager.getInstance(context)
    manager.beginWith(a)
    val continuation = manager.beginWith(a).then(b)
    println(continuation)
    WorkManager.getInstance(context).beginUniqueWork("name", androidx.work.ExistingWorkPolicy.KEEP, a)
}
