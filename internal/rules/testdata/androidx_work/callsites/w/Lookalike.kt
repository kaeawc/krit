package com.example.lookalike

class Constraints {
    class Builder {
        fun setRequiresDeviceIdle(value: Boolean) = this
        fun setRequiresCharging(value: Boolean) = this
    }
}

fun local() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
}
