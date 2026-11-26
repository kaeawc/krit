package com.example

import androidx.annotation.ReturnThis

open class Builder {
    @ReturnThis
    open fun name(name: String): Builder {
        val transform = { s: String -> s.trim() }
        listOf(1).forEach { return@forEach }
        fun local(): Int = 1
        val obj = object { fun other(): Any = Builder() }
        transform(name)
        return this
    }

    @ReturnThis
    open fun self(): Builder = this

    fun notAnnotated(): Builder = Builder()
}

class SubBuilder : Builder() {
    override fun name(name: String): Builder = this
}
