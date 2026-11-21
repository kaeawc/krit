package org.awaitility

object Awaitility {
    fun await(): ConditionFactory = TODO()
}
class ConditionFactory {
    fun until(condition: () -> Boolean) = Unit
}
