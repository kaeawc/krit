package com.example.look

import android.content.Context

// A local type that shares the simple name of androidx.startup.Initializer.
interface Initializer<T> {
    fun create(context: Context): T
}

class LocalLookalikeInitializer(private val key: String) : Initializer<Unit> {
    override fun create(context: Context) = Unit
}
