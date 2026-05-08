package com.example.releaseengineering

import timber.log.Timber

class UserService {
    fun load() {
        try {
            doWork()
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    private fun doWork() {}
}
