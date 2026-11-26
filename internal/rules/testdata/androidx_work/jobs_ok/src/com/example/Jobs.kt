package com.example

import android.app.job.JobParameters
import android.app.job.JobService
import androidx.work.Configuration

class SyncJobService : JobService() {
    override fun onStartJob(params: JobParameters?) = false
    override fun onStopJob(params: JobParameters?) = false
}

fun config() = Configuration.Builder().setJobSchedulerJobIdRange(1000, 2000).build()
