package com.example

import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

fun scopes(wm: WorkManager, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    wm.beginWith(a).also { it.enqueue() }
    wm.beginWith(a).apply { enqueue() }
    wm.beginWith(a).let { it.then(b).enqueue() }
    wm.beginWith(b).run { enqueue() }
}

fun assigned(wm: WorkManager, a: OneTimeWorkRequest) {
    val c: WorkContinuation
    c = wm.beginWith(a)
    c.enqueue()
}

fun leaked(wm: WorkManager, a: OneTimeWorkRequest) {
    wm.beginWith(a).also { println(it) }
}
