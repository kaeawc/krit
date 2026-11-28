package com.example

import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

fun then(continuation: WorkContinuation) {}

// The file's then() returns Unit, so the continuation is not returned.
fun localThen(wm: WorkManager, a: OneTimeWorkRequest) = then(wm.beginWith(a))

fun reassigned(wm: WorkManager, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    var c = wm.beginWith(a)
    c = wm.beginWith(b)
    c.enqueue()
}

// The loop variable and the local hide the lambda's parameter.
fun loopShadow(wm: WorkManager, a: OneTimeWorkRequest, others: List<WorkContinuation>) {
    wm.beginWith(a).also { c -> for (c in others) { c.enqueue() } }
}

fun localShadow(wm: WorkManager, a: OneTimeWorkRequest, other: WorkContinuation) {
    wm.beginWith(a).also { c ->
        val c = other
        c.enqueue()
    }
}

fun lambdaParam(wm: WorkManager, a: OneTimeWorkRequest) {
    wm.beginWith(a).also { c -> c.enqueue() }
}

// The assignment's right side still reads the first continuation.
fun thenReassigned(wm: WorkManager, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
    var c = wm.beginWith(a)
    c = c.then(b)
    c.enqueue()
}
