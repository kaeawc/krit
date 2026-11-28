package com.example

import android.content.Context
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

fun schedule(context: Context, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    val manager = WorkManager.getInstance(context)
    val c1 = manager.beginWith(a)
    val c2 = manager.beginWith(b)
    WorkContinuation.combine(listOf(c1, c2)).enqueue()
    // The pre-1.0 synchronous().enqueueSync(), which Lint's detector still names; it does not resolve against 2.10.0.
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
