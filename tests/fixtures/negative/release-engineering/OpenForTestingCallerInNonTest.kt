package com.example

@OpenForTesting
open class BaseForTests

fun mentionsOnly() {
    val text = "class Fake : BaseForTests()"
    // class Commented : BaseForTests()
    BaseForTests()
}

open class UnresolvedBase

class ProductionSubclass : UnresolvedBase()
