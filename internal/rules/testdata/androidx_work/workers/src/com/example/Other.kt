package com.example

import android.content.Context
import androidx.work.WorkerParameters

private class CrossFileChild(context: Context, params: WorkerParameters) : SameFileBase(context, params) {
    override fun doWork() = Result.success()
}
