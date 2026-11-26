package com.example

import android.content.Context
import androidx.startup.Initializer

class ListedInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = listOf(ReachableInitializer::class.java)
}

class ReachableInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class UnlistedInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class RelativeInitializer : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class WrongValue : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class ArgsInitializer(val name: String) : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class DefaultArgsInitializer(val name: String = "x") : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class SecondaryInitializer(val name: String) : Initializer<Unit> {
    constructor() : this("default")
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

abstract class BaseInitializer : Initializer<Unit> {
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}

class DerivedInitializer : BaseInitializer() {
    override fun create(context: Context) = Unit
}

interface MarkerInitializer : Initializer<Unit>

class NotAnInitializer(val x: Int)
