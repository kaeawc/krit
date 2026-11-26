package com.example

import android.content.Context
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

fun schedule(context: Context, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    val manager = WorkManager.getInstance(context)
    manager.beginWith(a).enqueue()
    val c = manager.beginWith(a)
    c.then(b).enqueue()
    val c1 = manager.beginWith(a)
    WorkContinuation.combine(c1).enqueue()
    manager.beginWith(a).synchronous().enqueueSync()
}

fun build(context: Context, a: OneTimeWorkRequest): WorkContinuation =
    WorkManager.getInstance(context).beginWith(a)

class Lookalike {
    fun beginWith(x: Any) = this
}

fun lookalike() {
    Lookalike().beginWith(1)
}
