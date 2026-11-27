package com.example

import android.content.Context
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkContinuation
import androidx.work.WorkManager

class Promise {
    fun then(x: Int): Promise = this
}

class Fake {
    fun beginWith(x: OneTimeWorkRequest): Fake = this
}

class Worker(private val p: Promise) {
    // then() on something that is no WorkContinuation.
    fun chain() {
        val next = p.then(1)
        next.then(2)
    }
}

class Shadows(private val context: Context) {
    fun lambdaParam(fakes: List<Fake>, a: OneTimeWorkRequest) {
        val manager = WorkManager.getInstance(context)
        fakes.forEach { manager -> manager.beginWith(a) }
        manager.beginWith(a).enqueue()
    }

    fun loopVariable(fakes: List<Fake>, a: OneTimeWorkRequest) {
        val manager = WorkManager.getInstance(context)
        for (manager in fakes) {
            manager.beginWith(a)
        }
        manager.beginWith(a).enqueue()
    }
}

class Nested(private val context: Context) {
    fun alsoOther(a: OneTimeWorkRequest, others: List<WorkContinuation>) {
        WorkManager.getInstance(context).beginWith(a).also { others.forEach { it.enqueue() } }
    }

    fun lambdaRebinds(a: OneTimeWorkRequest, others: List<WorkContinuation>) {
        val c = WorkManager.getInstance(context).beginWith(a)
        others.forEach { c -> c.enqueue() }
    }

    fun localFunction(a: OneTimeWorkRequest) {
        val c = WorkManager.getInstance(context).beginWith(a)
        fun inner(c: WorkContinuation) {
            c.enqueue()
        }
        println(c)
    }

    init {
        WorkManager.getInstance(context).beginWith(OneTimeWorkRequest.from(androidx.work.ListenableWorker::class.java))
    }
}
