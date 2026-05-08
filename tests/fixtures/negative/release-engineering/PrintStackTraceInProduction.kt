package com.example.releaseengineering

import timber.log.Timber

class UserService {
    fun load() {
        try {
            doWork()
        } catch (e: Exception) {
            Timber.e(e, "load failed")
        }
    }

    private fun doWork() {}
}
