package com.example

sealed interface Result
data class Success(val value: Int) : Result
@JvmRecord data class Point(val x: Int, val y: Int)
open class Repository
