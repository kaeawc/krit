package com.example

import androidx.work.Constraints

fun sameVariable() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
}

fun reversedOrder() {
    val builder = Constraints.Builder()
    builder.setRequiresCharging(true)
    builder.setRequiresDeviceIdle(true)
}

fun chained(): Constraints =
    Constraints.Builder()
        .setRequiresDeviceIdle(true)
        .setRequiresCharging(true)
        .build()

fun chargingFalse() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(false)
}

fun idleFalse() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(false)
    builder.setRequiresCharging(true)
}

fun differentBuilders() {
    val idle = Constraints.Builder()
    val charging = Constraints.Builder()
    idle.setRequiresDeviceIdle(true)
    charging.setRequiresCharging(true)
}

fun twice() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
    builder.setRequiresCharging(true)
}

fun nestedLambda() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    run {
        builder.setRequiresCharging(true)
    }
}

const val IDLE = true

fun constantArgument() {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(IDLE)
    builder.setRequiresCharging(true)
}

fun splitAcrossFunctionsIdle(builder: Constraints.Builder) {
    builder.setRequiresDeviceIdle(true)
}

fun splitAcrossFunctionsCharging(builder: Constraints.Builder) {
    builder.setRequiresCharging(true)
}
