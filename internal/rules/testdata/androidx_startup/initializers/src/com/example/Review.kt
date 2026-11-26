package com.example

import android.content.Context
import androidx.startup.Initializer

class SecOverloads : Initializer<Unit> {
    @JvmOverloads constructor(a: Int = 1)
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class CommentCtor(val x: Int) : Initializer<Unit> {
    constructor(/* none */) : this(0)
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}
