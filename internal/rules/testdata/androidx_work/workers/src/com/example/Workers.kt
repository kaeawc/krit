package com.example

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.ListenableWorker
import androidx.work.Worker
import androidx.work.WorkerParameters

class PublicWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

internal class InternalWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

private class PrivateWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

class Outer {
    class NestedWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
        override suspend fun doWork() = Result.success()
    }

    private class PrivateNestedWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
        override suspend fun doWork() = Result.success()
    }

    protected class ProtectedNestedWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
        override suspend fun doWork() = Result.success()
    }
}

private abstract class AbstractWorker(context: Context, params: WorkerParameters) : ListenableWorker(context, params)

private class ChildWorker(context: Context, params: WorkerParameters) : SameFileBase(context, params) {
    override fun doWork() = Result.success()
}

abstract class SameFileBase(context: Context, params: WorkerParameters) : Worker(context, params)

class NotAWorker
