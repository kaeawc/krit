package com.example

import androidx.lifecycle.Lifecycle
import androidx.lifecycle.testing.TestLifecycleOwner
import kotlinx.coroutines.test.runTest

const val OTHER = 1
var counter = 0

fun direct() = runTest {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.RESUMED
}

fun suspending() = runTest {
    val owner = TestLifecycleOwner()
    owner.setCurrentState(Lifecycle.State.RESUMED)
}

fun imported() = runTest(testBody = {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.STARTED
})

fun resetByLaterAssignment() = runTest {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.CREATED
    counter = OTHER
}

fun comparison() = runTest {
    val owner = TestLifecycleOwner()
    if (owner.currentState == Lifecycle.State.CREATED) println()
}

fun outside() {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.DESTROYED
}

fun nested() = runTest {
    val owner = TestLifecycleOwner()
    listOf(1).forEach { owner.currentState = Lifecycle.State.STARTED }
}
