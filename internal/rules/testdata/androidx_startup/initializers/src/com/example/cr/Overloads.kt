package com.example.cr

import android.content.Context
import androidx.startup.Initializer

class TextOverloads : Initializer<Unit> {
    constructor(s: String = "@JvmOverloads")
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class Root : Initializer<Unit> {
    class NestedChild : Initializer<Unit> {
        override fun create(context: Context) = Unit
        override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
    }

    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = listOf(NestedChild::class.java)
}
