package com.example

import android.content.Context
import android.util.Log
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

class Flow(private val context: Context) {
    private val wm = WorkManager.getInstance(context)

    fun alsoThenEnqueue(a: OneTimeWorkRequest) {
        wm.beginWith(a).also { Log.d("t", "x") }.enqueue()
    }

    fun applyThenEnqueue(a: OneTimeWorkRequest) {
        val c = wm.beginWith(a).apply { Log.d("t", "x") }
        c.enqueue()
    }

    fun takeIfEnqueue(a: OneTimeWorkRequest) {
        wm.beginWith(a).takeIf { true }?.enqueue()
    }

    fun withEnqueue(a: OneTimeWorkRequest) {
        val c = wm.beginWith(a)
        with(c) { enqueue() }
    }

    fun alias(a: OneTimeWorkRequest) {
        val c = wm.beginWith(a)
        val d = c
        d.enqueue()
    }

    fun ifBranches(x: Boolean, a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
        val c = if (x) wm.beginWith(a) else wm.beginWith(b)
        c.enqueue()
    }

    fun elvis(other: WorkContinuation?, a: OneTimeWorkRequest) {
        val c = other ?: wm.beginWith(a)
        c.enqueue()
    }

    // The inner c is another variable: the outer one is never enqueued.
    fun shadowedInRun(a: OneTimeWorkRequest, b: OneTimeWorkRequest) {
        val c = wm.beginWith(a)
        run {
            val c = wm.beginWith(b)
            c.enqueue()
        }
    }

    private var field: WorkContinuation? = null

    // Assigned to a field: the message names no variable.
    fun assignField(a: OneTimeWorkRequest) {
        field = wm.beginWith(a)
    }

    fun assignLocal(a: OneTimeWorkRequest) {
        var c: WorkContinuation
        c = wm.beginWith(a)
        println(c)
    }
}
