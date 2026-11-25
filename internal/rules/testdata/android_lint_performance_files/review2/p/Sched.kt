// @file:JvmName("Fake")
package p

private fun helper() = 1
private fun work() = 3

open class Base {
    fun work() = 4
}

class Locals {
    fun call(): Int {
        val helper = { 2 }
        return helper()
    }
}

class Child : Base() {
    companion object {
        private fun work() = 5
    }
    fun call() = work()
}

class Uses {
    fun x() = helper()
}
