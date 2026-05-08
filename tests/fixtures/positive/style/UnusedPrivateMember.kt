package com.example.style

class UnusedPrivateMemberPositive {
    private val unusedProperty = "never used"

    private fun unusedFunction() {
        println("never called")
    }

    fun publicFunction() {
        println("public")
    }
}
