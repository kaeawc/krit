package com.example.style

class UnusedPrivateMemberNegative {
    private val usedProperty = "used"

    private fun usedFunction() {
        println("called")
    }

    fun publicFunction() {
        println(usedProperty)
        usedFunction()
    }
}
