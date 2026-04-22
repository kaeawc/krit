package com.example

@OpenForTesting(
    reason = "subclassed by tests"
)
open class BaseForTests<T>

class ProductionSubclass :
    Runnable,
    BaseForTests<String>()

class Outer {
    @OpenForTesting
    open class NestedBase

    class NestedProductionSubclass : NestedBase()
}
